package transport

import (
	"context"

	"github.com/elastic/apm-agent-go/model"
)

// Transport provides an interface for sending transactions and errors
// payloads to Elastic APM. Methods are not required to be safe for
// concurrent use; tracers should serialize the calls.
type Transport interface {
	// SendTransactions sends the transactions payload to the server.
	SendTransactions(context.Context, *model.TransactionsPayload) error

	// SendErrors sends the errors payload to the server.
	SendErrors(context.Context, *model.ErrorsPayload) error
}
