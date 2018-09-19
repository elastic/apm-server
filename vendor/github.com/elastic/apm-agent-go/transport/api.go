package transport

import (
	"context"

	"github.com/elastic/apm-agent-go/model"
)

// Transport provides an interface for sending data to the Elastic APM
// server. Methods are not required to be safe for concurrent use.
type Transport interface {
	// SendErrors sends the errors payload to the server.
	SendErrors(context.Context, *model.ErrorsPayload) error

	// SendMetrics sends the metrics payload to the server.
	SendMetrics(context.Context, *model.MetricsPayload) error

	// SendTransactions sends the transactions payload to the server.
	SendTransactions(context.Context, *model.TransactionsPayload) error
}
