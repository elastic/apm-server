package apm

import (
	"go.elastic.co/apm/internal/ringbuffer"
	"go.elastic.co/apm/model"
	"go.elastic.co/apm/stacktrace"
	"go.elastic.co/fastjson"
)

const (
	transactionBlockTag ringbuffer.BlockTag = iota + 1
	spanBlockTag
	errorBlockTag
	metricsBlockTag
)

// notSampled is used as the pointee for the model.Transaction.Sampled field
// of non-sampled transactions.
var notSampled = false

type modelWriter struct {
	buffer          *ringbuffer.Buffer
	metricsBuffer   *ringbuffer.Buffer
	cfg             *tracerConfig
	stats           *TracerStats
	json            fastjson.Writer
	modelStacktrace []model.StacktraceFrame
}

// writeTransaction encodes tx as JSON to the buffer, and then resets tx.
func (w *modelWriter) writeTransaction(tx *TransactionData) {
	var modelTx model.Transaction
	w.buildModelTransaction(&modelTx, tx)
	w.json.RawString(`{"transaction":`)
	modelTx.MarshalFastJSON(&w.json)
	w.json.RawByte('}')
	w.buffer.WriteBlock(w.json.Bytes(), transactionBlockTag)
	w.json.Reset()
	tx.reset()
}

// writeSpan encodes s as JSON to the buffer, and then resets s.
func (w *modelWriter) writeSpan(s *SpanData) {
	var modelSpan model.Span
	w.buildModelSpan(&modelSpan, s)
	w.json.RawString(`{"span":`)
	modelSpan.MarshalFastJSON(&w.json)
	w.json.RawByte('}')
	w.buffer.WriteBlock(w.json.Bytes(), spanBlockTag)
	w.json.Reset()
	s.reset()
}

// writeError encodes e as JSON to the buffer, and then resets e.
func (w *modelWriter) writeError(e *ErrorData) {
	var modelError model.Error
	w.buildModelError(&modelError, e)
	w.json.RawString(`{"error":`)
	modelError.MarshalFastJSON(&w.json)
	w.json.RawByte('}')
	w.buffer.WriteBlock(w.json.Bytes(), errorBlockTag)
	w.json.Reset()
	e.reset()
}

// writeMetrics encodes m as JSON to the w.metricsBuffer, and then resets m.
//
// Note that we do not write metrics to the main ring buffer (w.buffer), as
// periodic metrics would be evicted by transactions/spans in a busy system.
func (w *modelWriter) writeMetrics(m *Metrics) {
	for _, m := range m.metrics {
		w.json.RawString(`{"metricset":`)
		m.MarshalFastJSON(&w.json)
		w.json.RawString("}")
		w.metricsBuffer.WriteBlock(w.json.Bytes(), metricsBlockTag)
		w.json.Reset()
	}
	m.reset()
}

func (w *modelWriter) buildModelTransaction(out *model.Transaction, tx *TransactionData) {
	out.ID = model.SpanID(tx.traceContext.Span)
	out.TraceID = model.TraceID(tx.traceContext.Trace)
	out.ParentID = model.SpanID(tx.parentSpan)

	out.Name = truncateString(tx.Name)
	out.Type = truncateString(tx.Type)
	out.Result = truncateString(tx.Result)
	out.Timestamp = model.Time(tx.timestamp.UTC())
	out.Duration = tx.Duration.Seconds() * 1000
	out.SpanCount.Started = tx.spansCreated
	out.SpanCount.Dropped = tx.spansDropped

	if !tx.traceContext.Options.Recorded() {
		out.Sampled = &notSampled
	}

	out.Context = tx.Context.build()
	if len(w.cfg.sanitizedFieldNames) != 0 && out.Context != nil {
		if out.Context.Request != nil {
			sanitizeRequest(out.Context.Request, w.cfg.sanitizedFieldNames)
		}
		if out.Context.Response != nil {
			sanitizeResponse(out.Context.Response, w.cfg.sanitizedFieldNames)
		}
	}
}

func (w *modelWriter) buildModelSpan(out *model.Span, span *SpanData) {
	w.modelStacktrace = w.modelStacktrace[:0]
	out.ID = model.SpanID(span.traceContext.Span)
	out.TraceID = model.TraceID(span.traceContext.Trace)
	out.ParentID = model.SpanID(span.parentID)
	out.TransactionID = model.SpanID(span.transactionID)

	out.Name = truncateString(span.Name)
	out.Type = truncateString(span.Type)
	out.Subtype = truncateString(span.Subtype)
	out.Action = truncateString(span.Action)
	out.Timestamp = model.Time(span.timestamp.UTC())
	out.Duration = span.Duration.Seconds() * 1000
	out.Context = span.Context.build()

	w.modelStacktrace = appendModelStacktraceFrames(w.modelStacktrace, span.stacktrace)
	out.Stacktrace = w.modelStacktrace
	w.setStacktraceContext(out.Stacktrace)
}

func (w *modelWriter) buildModelError(out *model.Error, e *ErrorData) {
	out.ID = model.TraceID(e.ID)
	out.TraceID = model.TraceID(e.TraceID)
	out.ParentID = model.SpanID(e.ParentID)
	out.TransactionID = model.SpanID(e.TransactionID)
	out.Timestamp = model.Time(e.Timestamp.UTC())
	out.Context = e.Context.build()
	out.Culprit = e.Culprit

	w.modelStacktrace = w.modelStacktrace[:0]
	if len(e.stacktrace) != 0 {
		w.modelStacktrace = appendModelStacktraceFrames(w.modelStacktrace, e.stacktrace)
		w.setStacktraceContext(w.modelStacktrace)
	}

	if e.exception.message != "" {
		out.Exception = model.Exception{
			Message: e.exception.message,
			Code: model.ExceptionCode{
				String: e.exception.codeString,
				Number: e.exception.codeNumber,
			},
			Type:       e.exception.typeName,
			Module:     e.exception.typePackagePath,
			Handled:    e.Handled,
			Stacktrace: w.modelStacktrace[:e.exceptionStacktraceFrames],
		}
		if len(e.exception.attrs) != 0 {
			out.Exception.Attributes = e.exception.attrs
		}
		if out.Culprit == "" {
			out.Culprit = stacktraceCulprit(out.Exception.Stacktrace)
		}
	}
	if e.log.Message != "" {
		out.Log = model.Log{
			Message:      e.log.Message,
			Level:        e.log.Level,
			LoggerName:   e.log.LoggerName,
			ParamMessage: e.log.MessageFormat,
			Stacktrace:   w.modelStacktrace[e.exceptionStacktraceFrames:],
		}
		if out.Culprit == "" {
			out.Culprit = stacktraceCulprit(out.Log.Stacktrace)
		}
	}
}

func stacktraceCulprit(frames []model.StacktraceFrame) string {
	for _, frame := range frames {
		if !frame.LibraryFrame {
			return frame.Function
		}
	}
	return ""
}

func (w *modelWriter) setStacktraceContext(stack []model.StacktraceFrame) {
	if w.cfg.contextSetter == nil || len(stack) == 0 {
		return
	}
	err := stacktrace.SetContext(w.cfg.contextSetter, stack, w.cfg.preContext, w.cfg.postContext)
	if err != nil {
		if w.cfg.logger != nil {
			w.cfg.logger.Debugf("setting context failed: %v", err)
		}
		w.stats.Errors.SetContext++
	}
}
