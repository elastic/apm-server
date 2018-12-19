package apm

import (
	"context"

	"go.elastic.co/apm/internal/apmcontext"
)

// ContextWithSpan returns a copy of parent in which the given span
// is stored, associated with the key ContextSpanKey.
func ContextWithSpan(parent context.Context, s *Span) context.Context {
	return apmcontext.ContextWithSpan(parent, s)
}

// ContextWithTransaction returns a copy of parent in which the given
// transaction is stored, associated with the key ContextTransactionKey.
func ContextWithTransaction(parent context.Context, t *Transaction) context.Context {
	return apmcontext.ContextWithTransaction(parent, t)
}

// SpanFromContext returns the current Span in context, if any. The span must
// have been added to the context previously using ContextWithSpan, or the
// top-level StartSpan function.
func SpanFromContext(ctx context.Context) *Span {
	value, _ := apmcontext.SpanFromContext(ctx).(*Span)
	return value
}

// TransactionFromContext returns the current Transaction in context, if any.
// The transaction must have been added to the context previously using
// ContextWithTransaction.
func TransactionFromContext(ctx context.Context) *Transaction {
	value, _ := apmcontext.TransactionFromContext(ctx).(*Transaction)
	return value
}

// StartSpan is equivalent to calling StartSpanOptions with a zero SpanOptions struct.
func StartSpan(ctx context.Context, name, spanType string) (*Span, context.Context) {
	return StartSpanOptions(ctx, name, spanType, SpanOptions{})
}

// StartSpanOptions starts and returns a new Span within the sampled transaction
// and parent span in the context, if any. If the span isn't dropped, it will be
// stored in the resulting context.
//
// If opts.Parent is non-zero, its value will be used in preference to any parent
// span in ctx.
//
// StartSpanOptions always returns a non-nil Span. Its End method must be called
// when the span completes.
func StartSpanOptions(ctx context.Context, name, spanType string, opts SpanOptions) (*Span, context.Context) {
	tx := TransactionFromContext(ctx)
	opts.parent = SpanFromContext(ctx)
	span := tx.StartSpanOptions(name, spanType, opts)
	if !span.Dropped() {
		ctx = ContextWithSpan(ctx, span)
	}
	return span, ctx
}

// CaptureError returns a new Error related to the sampled transaction
// and span present in the context, if any, and sets its exception info
// from err. The Error.Handled field will be set to true, and a stacktrace
// set either from err, or from the caller.
//
// If there is no span or transaction in the context, CaptureError returns
// nil. As a convenience, if the provided error is nil, then CaptureError
// will also return nil.
func CaptureError(ctx context.Context, err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if span := SpanFromContext(ctx); span != nil {
		span.mu.RLock()
		if !span.ended() {
			e = span.tracer.NewError(err)
			e.setSpanData(span.SpanData)
		}
		span.mu.RUnlock()
	} else if tx := TransactionFromContext(ctx); tx != nil {
		tx.mu.RLock()
		if !tx.ended() {
			e = tx.tracer.NewError(err)
			e.setTransactionData(tx.TransactionData)
		}
		tx.mu.RUnlock()
	}
	if e != nil {
		e.Handled = true
	}
	return e
}
