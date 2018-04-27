package transporttest

import (
	"context"

	"github.com/elastic/apm-agent-go/model"
)

// CallbackTransport is a transport that invokes the given
// callbacks for the payloads for each method call.
type CallbackTransport struct {
	Transactions func(context.Context, *model.TransactionsPayload) error
	Errors       func(context.Context, *model.ErrorsPayload) error
	Metrics      func(context.Context, *model.MetricsPayload) error
}

// SendTransactions returns t.Transactions(ctx, p).
func (t CallbackTransport) SendTransactions(ctx context.Context, p *model.TransactionsPayload) error {
	return t.Transactions(ctx, p)
}

// SendErrors returns t.Errors(ctx, p).
func (t CallbackTransport) SendErrors(ctx context.Context, p *model.ErrorsPayload) error {
	return t.Errors(ctx, p)
}

// SendMetrics returns t.Metrics(ctx, p).
func (t CallbackTransport) SendMetrics(ctx context.Context, p *model.MetricsPayload) error {
	return t.Metrics(ctx, p)
}
