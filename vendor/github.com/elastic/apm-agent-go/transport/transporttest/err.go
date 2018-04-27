package transporttest

import (
	"context"

	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport"
)

// Discard is a transport.Transport which discards
// all payloads, and returns no errors.
var Discard transport.Transport = ErrorTransport{}

// ErrorTransport is a transport that returns the stored error
// for each method call.
type ErrorTransport struct {
	Error error
}

// SendTransactions discards the payload and returns t.Error.
func (t ErrorTransport) SendTransactions(context.Context, *model.TransactionsPayload) error {
	return t.Error
}

// SendErrors discards the payload and returns t.Error.
func (t ErrorTransport) SendErrors(context.Context, *model.ErrorsPayload) error {
	return t.Error
}

// SendMetrics discards the payload and returns t.Error.
func (t ErrorTransport) SendMetrics(context.Context, *model.MetricsPayload) error {
	return t.Error
}
