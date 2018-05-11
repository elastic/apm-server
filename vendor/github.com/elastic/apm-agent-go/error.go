package elasticapm

import (
	"fmt"
	"net"
	"os"
	"reflect"
	"syscall"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-agent-go/internal/uuid"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
)

// Recover recovers panics, sending them as errors to
// the Elastic APM server. Recover is expected to be
// used in a deferred call.
//
// Recover simply calls t.Recovered(v, tx),
// where v is the recovered value, and then calls the
// resulting Error's Send method.
func (t *Tracer) Recover(tx *Transaction) {
	v := recover()
	if v == nil {
		return
	}
	t.Recovered(v, tx).Send()
}

// Recovered creates an Error with t.NewError(err), where
// err is either v (if v implements error), or otherwise
// fmt.Errorf("%v", v). The value v is expected to have
// come from a panic.
//
// The resulting error's Transaction will be set to tx,
func (t *Tracer) Recovered(v interface{}, tx *Transaction) *Error {
	var e *Error
	switch v := v.(type) {
	case error:
		e = t.NewError(v)
	default:
		e = t.NewError(fmt.Errorf("%v", v))
	}
	e.Transaction = tx
	return e
}

// NewError returns a new Error with details taken from err.
// NewError will panic if called with a nil error.
//
// The exception message will be set to err.Error().
// The exception module and type will be set to the package
// and type name of the cause of the error, respectively,
// where the cause has the same definition as given by
// github.com/pkg/errors.
//
// If err implements
//   type interface {
//       StackTrace() github.com/pkg/errors.StackTrace
//   }
// or
//   type interface {
//       StackTrace() []stacktrace.Frame
//   }
// then one of those will be used to set the error
// stacktrace. Otherwise, NewError will take a stacktrace.
//
// If err implements
//   type interface {Type() string}
// then that will be used to set the error type.
//
// If err implements
//   type interface {Code() string}
// or
//   type interface {Code() float64}
// then one of those will be used to set the error code.
func (t *Tracer) NewError(err error) *Error {
	if err == nil {
		panic("NewError must be called with a non-nil error")
	}
	e := t.newError()
	if uuid, err := uuid.NewV4(); err == nil {
		e.ID = uuid.String()
	}
	e.model.Exception.Message = err.Error()
	if e.model.Exception.Message == "" {
		e.model.Exception.Message = "[EMPTY]"
	}
	initException(&e.model.Exception, errors.Cause(err))
	initStacktrace(e, err)
	if e.stacktrace == nil {
		e.SetStacktrace(2)
	}
	return e
}

// NewErrorLog returns a new Error for the given ErrorLogRecord.
//
// The resulting Error's stacktrace will not be set. Call the
// SetStacktrace method to set it, if desired.
//
// If r.Message is empty, "[EMPTY]" will be used.
func (t *Tracer) NewErrorLog(r ErrorLogRecord) *Error {
	e := t.newError()
	e.model.Log = model.Log{
		Message:      r.Message,
		Level:        truncateString(r.Level),
		LoggerName:   truncateString(r.LoggerName),
		ParamMessage: truncateString(r.MessageFormat),
	}
	if e.model.Log.Message == "" {
		e.model.Log.Message = "[EMPTY]"
	}
	return e
}

// newError returns a new Error associated with the Tracer.
func (t *Tracer) newError() *Error {
	e, _ := t.errorPool.Get().(*Error)
	if e == nil {
		e = &Error{
			tracer: t,
			Context: Context{
				captureBodyMask: CaptureBodyErrors,
			},
		}
	}
	e.Timestamp = time.Now()
	return e
}

// Error describes an error occurring in the monitored service.
type Error struct {
	model           model.Error
	tracer          *Tracer
	stacktrace      []stacktrace.Frame
	modelStacktrace []model.StacktraceFrame

	// ID is the unique ID of the error. This is set by NewError,
	// and can be used for correlating errors and logs records.
	ID string

	// Culprit is the name of the function that caused the error.
	//
	// This is initially unset; if it remains unset by the time
	// Send is invoked, and the error has a stacktrace, the first
	// non-library frame in the stacktrace will be considered the
	// culprit.
	Culprit string

	// Transaction is the transaction to which the error correspoonds,
	// if any. If this is set, the error's Send method must be called
	// before the transaction's End method.
	Transaction *Transaction

	// Timestamp records the time at which the error occurred.
	// This is set when the Error object is created, but may
	// be overridden any time before the Send method is called.
	Timestamp time.Time

	// Handled records whether or not the error was handled. This
	// only applies to "exception" errors (errors created with
	// NewError, or Recovered), and is ignored by "log" errors.
	Handled bool

	// Context holds the context for this error.
	Context Context
}

func (e *Error) reset() {
	*e = Error{
		tracer:          e.tracer,
		stacktrace:      e.stacktrace[:0],
		modelStacktrace: e.modelStacktrace[:0],
		Context:         e.Context,
	}
	e.Context.reset()
}

// Send enqueues the error for sending to the Elastic APM server.
// The Error must not be used after this.
func (e *Error) Send() {
	select {
	case e.tracer.errors <- e:
	default:
		// Enqueuing an error should never block.
		e.tracer.statsMu.Lock()
		e.tracer.stats.ErrorsDropped++
		e.tracer.statsMu.Unlock()
		e.reset()
		e.tracer.errorPool.Put(e)
	}
}

func (e *Error) setStacktrace() {
	if len(e.stacktrace) == 0 {
		return
	}
	e.modelStacktrace = appendModelStacktraceFrames(e.modelStacktrace, e.stacktrace)
	e.model.Log.Stacktrace = e.modelStacktrace
	e.model.Exception.Stacktrace = e.modelStacktrace
}

func (e *Error) setCulprit() {
	if e.Culprit != "" {
		e.model.Culprit = e.Culprit
	} else if e.modelStacktrace != nil {
		e.model.Culprit = stacktraceCulprit(e.modelStacktrace)
	}
}

// stacktraceCulprit returns the first non-library stacktrace frame's
// function name.
func stacktraceCulprit(frames []model.StacktraceFrame) string {
	for _, frame := range frames {
		if !frame.LibraryFrame {
			return frame.Function
		}
	}
	return ""
}

func initException(e *model.Exception, err error) {
	setAttr := func(k string, v interface{}) {
		if e.Attributes == nil {
			e.Attributes = make(map[string]interface{})
		}
		e.Attributes[k] = v
	}

	// Set Module, Type, Attributes, and Code.
	switch err := err.(type) {
	case *net.OpError:
		e.Module, e.Type = "net", "OpError"
		setAttr("op", err.Op)
		setAttr("net", err.Net)
		setAttr("source", err.Source)
		setAttr("addr", err.Addr)
	case *os.LinkError:
		e.Module, e.Type = "os", "LinkError"
		setAttr("op", err.Op)
		setAttr("old", err.Old)
		setAttr("new", err.New)
	case *os.PathError:
		e.Module, e.Type = "os", "PathError"
		setAttr("op", err.Op)
		setAttr("path", err.Path)
	case *os.SyscallError:
		e.Module, e.Type = "os", "SyscallError"
		setAttr("syscall", err.Syscall)
	case syscall.Errno:
		e.Module, e.Type = "syscall", "Errno"
		e.Code.Number = float64(uintptr(err))
	default:
		t := reflect.TypeOf(err)
		if t.Name() == "" && t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		e.Module, e.Type = t.PkgPath(), t.Name()

		// If the error implements Type, use that to
		// override the type name determined through
		// reflection.
		if err, ok := err.(interface {
			Type() string
		}); ok {
			e.Type = err.Type()
		}

		// If the error implements a Code method, use
		// that to set the exception code.
		switch err := err.(type) {
		case interface {
			Code() string
		}:
			e.Code.String = err.Code()
		case interface {
			Code() float64
		}:
			e.Code.Number = err.Code()
		}
	}
	if errTemporary(err) {
		setAttr("temporary", true)
	}
	if errTimeout(err) {
		setAttr("timeout", true)
	}
	e.Code.String = truncateString(e.Code.String)
	e.Type = truncateString(e.Type)
}

func initStacktrace(e *Error, err error) {
	type internalStackTracer interface {
		StackTrace() []stacktrace.Frame
	}
	type errorsStackTracer interface {
		StackTrace() errors.StackTrace
	}
	switch stackTracer := err.(type) {
	case internalStackTracer:
		e.stacktrace = append(e.stacktrace[:0], stackTracer.StackTrace()...)
	case errorsStackTracer:
		stackTrace := stackTracer.StackTrace()
		pc := make([]uintptr, len(stackTrace))
		for i, frame := range stackTrace {
			pc[i] = uintptr(frame)
		}
		e.stacktrace = stacktrace.AppendCallerFrames(e.stacktrace[:0], pc)
	}
}

// SetStacktrace sets the stacktrace for the error,
// skipping the first skip number of frames, excluding
// the SetStacktrace function.
func (e *Error) SetStacktrace(skip int) {
	e.stacktrace = stacktrace.AppendStacktrace(e.stacktrace[:0], skip+1, -1)
}

func errTemporary(err error) bool {
	type temporaryError interface {
		Temporary() bool
	}
	terr, ok := err.(temporaryError)
	return ok && terr.Temporary()
}

func errTimeout(err error) bool {
	type timeoutError interface {
		Timeout() bool
	}
	terr, ok := err.(timeoutError)
	return ok && terr.Timeout()
}

// ErrorLogRecord holds details of an error log record.
type ErrorLogRecord struct {
	// Message holds the message for the log record,
	// e.g. "failed to connect to %s".
	//
	// If this is empty, "[EMPTY]" will be used.
	Message string

	// MessageFormat holds the non-interpolated format
	// of the log record, e.g. "failed to connect to %s".
	//
	// This is optional.
	MessageFormat string

	// Level holds the severity level of the log record.
	//
	// This is optional.
	Level string

	// LoggerName holds the name of the logger used.
	//
	// This is optional.
	LoggerName string
}
