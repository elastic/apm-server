package apm

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"reflect"
	"syscall"
	"time"

	"github.com/pkg/errors"

	"go.elastic.co/apm/stacktrace"
)

// Recovered creates an Error with t.NewError(err), where
// err is either v (if v implements error), or otherwise
// fmt.Errorf("%v", v). The value v is expected to have
// come from a panic.
func (t *Tracer) Recovered(v interface{}) *Error {
	var e *Error
	switch v := v.(type) {
	case error:
		e = t.NewError(v)
	default:
		e = t.NewError(fmt.Errorf("%v", v))
	}
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
	rand.Read(e.ID[:]) // ignore error, can't do anything about it
	initException(&e.exception, err)
	initStacktrace(e, err)
	if len(e.stacktrace) == 0 {
		e.SetStacktrace(2)
	}
	e.exceptionStacktraceFrames = len(e.stacktrace)
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
	e.log = ErrorLogRecord{
		Message:       truncateString(r.Message),
		MessageFormat: truncateString(r.MessageFormat),
		Level:         truncateString(r.Level),
		LoggerName:    truncateString(r.LoggerName),
	}
	if e.log.Message == "" {
		e.log.Message = "[EMPTY]"
	}
	rand.Read(e.ID[:]) // ignore error, can't do anything about it
	if r.Error != nil {
		initException(&e.exception, r.Error)
		initStacktrace(e, r.Error)
		e.exceptionStacktraceFrames = len(e.stacktrace)
	}
	return e
}

// newError returns a new Error associated with the Tracer.
func (t *Tracer) newError() *Error {
	e, _ := t.errorDataPool.Get().(*ErrorData)
	if e == nil {
		e = &ErrorData{
			tracer: t,
			Context: Context{
				captureBodyMask: CaptureBodyErrors,
			},
		}
	}
	e.Timestamp = time.Now()
	return &Error{ErrorData: e}
}

// Error describes an error occurring in the monitored service.
type Error struct {
	// ErrorData holds the error data. This field is set to nil when
	// the error's Send method is called.
	*ErrorData
}

// ErrorData holds the details for an error, and is embedded inside Error.
// When the error is sent, its ErrorData field will be set to nil.
type ErrorData struct {
	tracer     *Tracer
	stacktrace []stacktrace.Frame
	exception  exceptionData
	log        ErrorLogRecord

	// exceptionStacktraceFrames holds the number of stacktrace
	// frames for the exception; stacktrace may hold frames for
	// both the exception and the log record.
	exceptionStacktraceFrames int

	// ID is the unique identifier of the error. This is set by
	// the various error constructors, and is exposed only so
	// the error ID can be logged or displayed to the user.
	ID ErrorID

	// TraceID is the unique identifier of the trace in which
	// this error occurred. If the error is not associated with
	// a trace, this will be the zero value.
	TraceID TraceID

	// TransactionID is the unique identifier of the transaction
	// in which this error occurred. If the error is not associated
	// with a transaction, this will be the zero value.
	TransactionID SpanID

	// ParentID is the unique identifier of the transaction or span
	// in which this error occurred. If the error is not associated
	// with a transaction or span, this will be the zero value.
	ParentID SpanID

	// Culprit is the name of the function that caused the error.
	//
	// This is initially unset; if it remains unset by the time
	// Send is invoked, and the error has a stacktrace, the first
	// non-library frame in the stacktrace will be considered the
	// culprit.
	Culprit string

	// Timestamp records the time at which the error occurred.
	// This is set when the Error object is created, but may
	// be overridden any time before the Send method is called.
	Timestamp time.Time

	// Handled records whether or not the error was handled. This
	// is ignored by "log" errors with no associated error value.
	Handled bool

	// Context holds the context for this error.
	Context Context
}

// SetTransaction sets TraceID, TransactionID, and ParentID to the transaction's IDs.
//
// SetTransaction has no effect if called with an ended transaction.
func (e *Error) SetTransaction(tx *Transaction) {
	tx.mu.RLock()
	if !tx.ended() {
		e.setTransactionData(tx.TransactionData)
	}
	tx.mu.RUnlock()
}

func (e *Error) setTransactionData(td *TransactionData) {
	e.TraceID = td.traceContext.Trace
	e.ParentID = td.traceContext.Span
	e.TransactionID = e.ParentID
}

// SetSpan sets TraceID, TransactionID, and ParentID to the span's IDs. If you call
// this, it is not necessary to call SetTransaction.
//
// SetSpan has no effect if called with an ended span.
func (e *Error) SetSpan(s *Span) {
	s.mu.RLock()
	if !s.ended() {
		e.setSpanData(s.SpanData)
	}
	s.mu.RUnlock()
}

func (e *Error) setSpanData(s *SpanData) {
	e.TraceID = s.traceContext.Trace
	e.ParentID = s.traceContext.Span
	e.TransactionID = s.transactionID
}

// Send enqueues the error for sending to the Elastic APM server.
//
// Send will set e.ErrorData to nil, so the error must not be
// modified after Send returns.
func (e *Error) Send() {
	if e == nil || e.sent() {
		return
	}
	e.ErrorData.enqueue()
	e.ErrorData = nil
}

func (e *Error) sent() bool {
	return e.ErrorData == nil
}

func (e *ErrorData) enqueue() {
	select {
	case e.tracer.errors <- e:
	default:
		// Enqueuing an error should never block.
		e.tracer.statsMu.Lock()
		e.tracer.stats.ErrorsDropped++
		e.tracer.statsMu.Unlock()
		e.reset()
	}
}

func (e *ErrorData) reset() {
	*e = ErrorData{
		tracer:     e.tracer,
		stacktrace: e.stacktrace[:0],
		Context:    e.Context,
		exception:  e.exception,
	}
	e.Context.reset()
	e.exception.reset()
	e.tracer.errorDataPool.Put(e)
}

type exceptionData struct {
	message         string
	attrs           map[string]interface{}
	typeName        string
	typePackagePath string
	codeNumber      float64
	codeString      string
}

func (e *exceptionData) reset() {
	*e = exceptionData{
		attrs: e.attrs,
	}
	for k := range e.attrs {
		delete(e.attrs, k)
	}
}

func initException(e *exceptionData, err error) {
	setAttr := func(k string, v interface{}) {
		if e.attrs == nil {
			e.attrs = make(map[string]interface{})
		}
		e.attrs[k] = v
	}

	e.message = truncateString(err.Error())
	if e.message == "" {
		e.message = "[EMPTY]"
	}
	err = errors.Cause(err)

	// Set Module, Type, Attributes, and Code.
	switch err := err.(type) {
	case *net.OpError:
		e.typePackagePath, e.typeName = "net", "OpError"
		setAttr("op", err.Op)
		setAttr("net", err.Net)
		setAttr("source", err.Source)
		setAttr("addr", err.Addr)
	case *os.LinkError:
		e.typePackagePath, e.typeName = "os", "LinkError"
		setAttr("op", err.Op)
		setAttr("old", err.Old)
		setAttr("new", err.New)
	case *os.PathError:
		e.typePackagePath, e.typeName = "os", "PathError"
		setAttr("op", err.Op)
		setAttr("path", err.Path)
	case *os.SyscallError:
		e.typePackagePath, e.typeName = "os", "SyscallError"
		setAttr("syscall", err.Syscall)
	case syscall.Errno:
		e.typePackagePath, e.typeName = "syscall", "Errno"
		e.codeNumber = float64(uintptr(err))
	default:
		t := reflect.TypeOf(err)
		if t.Name() == "" && t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		e.typePackagePath, e.typeName = t.PkgPath(), t.Name()

		// If the error implements Type, use that to
		// override the type name determined through
		// reflection.
		if err, ok := err.(interface {
			Type() string
		}); ok {
			e.typeName = err.Type()
		}

		// If the error implements a Code method, use
		// that to set the exception code.
		switch err := err.(type) {
		case interface {
			Code() string
		}:
			e.codeString = err.Code()
		case interface {
			Code() float64
		}:
			e.codeNumber = err.Code()
		}
	}
	if errTemporary(err) {
		setAttr("temporary", true)
	}
	if errTimeout(err) {
		setAttr("timeout", true)
	}
	e.codeString = truncateString(e.codeString)
	e.typeName = truncateString(e.typeName)
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
		e.stacktrace = stacktrace.AppendCallerFrames(e.stacktrace[:0], pc, -1)
	}
}

// SetStacktrace sets the stacktrace for the error,
// skipping the first skip number of frames, excluding
// the SetStacktrace function.
func (e *Error) SetStacktrace(skip int) {
	retain := e.stacktrace[:0]
	if e.log.Message != "" {
		// This is a log error; the exception stacktrace
		// is unaffected by SetStacktrace in this case.
		retain = e.stacktrace[:e.exceptionStacktraceFrames]
	}
	e.stacktrace = stacktrace.AppendStacktrace(retain, skip+1, -1)
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

	// Error is an error associated with the log record.
	//
	// This is optional.
	Error error
}

// ErrorID uniquely identifies an error.
type ErrorID TraceID

// String returns id in its hex-encoded format.
func (id ErrorID) String() string {
	return TraceID(id).String()
}
