package apmcontext

import "context"

var (
	// ContextWithSpan takes a context and span and returns a new context
	// from which the span can be extracted using SpanFromContext.
	//
	// ContextWithSpan is used by apm.ContextWithSpan. It is a
	// variable to allow other packages, such as apmot, to replace it
	// at package init time.
	ContextWithSpan = DefaultContextWithSpan

	// ContextWithTransaction takes a context and transaction and returns
	// a new context from which the transaction can be extracted using
	// TransactionFromContext.
	//
	// ContextWithTransaction is used by apm.ContextWithTransaction.
	// It is a variable to allow other packages, such as apmot, to replace
	// it at package init time.
	ContextWithTransaction = DefaultContextWithTransaction

	// SpanFromContext returns a span included in the context using
	// ContextWithSpan.
	//
	// SpanFromContext is used by apm.SpanFromContext. It is a
	// variable to allow other packages, such as apmot, to replace it
	// at package init time.
	SpanFromContext = DefaultSpanFromContext

	// TransactionFromContext returns a transaction included in the context
	// using ContextWithTransaction.
	//
	// TransactionFromContext is used by apm.TransactionFromContext.
	// It is a variable to allow other packages, such as apmot, to replace
	// it at package init time.
	TransactionFromContext = DefaultTransactionFromContext
)

type spanKey struct{}
type transactionKey struct{}

// DefaultContextWithSpan is the default value for ContextWithSpan.
func DefaultContextWithSpan(ctx context.Context, span interface{}) context.Context {
	return context.WithValue(ctx, spanKey{}, span)
}

// DefaultContextWithTransaction is the default value for ContextWithTransaction.
func DefaultContextWithTransaction(ctx context.Context, tx interface{}) context.Context {
	return context.WithValue(ctx, transactionKey{}, tx)
}

// DefaultSpanFromContext is the default value for SpanFromContext.
func DefaultSpanFromContext(ctx context.Context) interface{} {
	return ctx.Value(spanKey{})
}

// DefaultTransactionFromContext is the default value for TransactionFromContext.
func DefaultTransactionFromContext(ctx context.Context) interface{} {
	return ctx.Value(transactionKey{})
}
