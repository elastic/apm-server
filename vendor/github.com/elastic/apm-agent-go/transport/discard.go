package transport

import (
	"context"

	"github.com/elastic/apm-agent-go/model"
)

type discardTransport struct {
	err error
}

func (t discardTransport) SendTransactions(context.Context, *model.TransactionsPayload) error {
	return t.err
}

func (t discardTransport) SendErrors(context.Context, *model.ErrorsPayload) error {
	return t.err
}
