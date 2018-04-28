package transporttest

import (
	"context"

	"github.com/elastic/apm-agent-go/model"
)

// ChannelTransport implements transport.Transport,
// sending payloads to the provided channels as
// request objects. Once a request object has been
// received, an error should be sent to its Result
// channel to unblock the tracer.
type ChannelTransport struct {
	Transactions chan<- SendTransactionsRequest
	Errors       chan<- SendErrorsRequest
	Metrics      chan<- SendMetricsRequest
}

// SendTransactionsRequest is the type of values sent over the
// ChannelTransport.Transactions channel when its SendTransactions
// method is called.
type SendTransactionsRequest struct {
	Payload *model.TransactionsPayload
	Result  chan<- error
}

// SendErrorsRequest is the type of values sent over the
// ChannelTransport.Errors channel when its SendErrors
// method is called.
type SendErrorsRequest struct {
	Payload *model.ErrorsPayload
	Result  chan<- error
}

// SendMetricsRequest is the type of values sent over the
// ChannelTransport.Metrics channel when its SendMetrics
// method is called.
type SendMetricsRequest struct {
	Payload *model.MetricsPayload
	Result  chan<- error
}

// SendTransactions sends a SendTransactionsRequest value over the
// c.Transactions channel with the given payload, and waits for a
// response on the error channel included in the request, or for
// the context to be canceled.
func (c *ChannelTransport) SendTransactions(ctx context.Context, payload *model.TransactionsPayload) error {
	result := make(chan error, 1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.Transactions <- SendTransactionsRequest{payload, result}:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-result:
			return err
		}
	}
}

// SendErrors sends a SendErrorsRequest value over the c.Errors channel
// with the given payload, and waits for a response on the error channel
// included in the request, or for the context to be canceled.
func (c *ChannelTransport) SendErrors(ctx context.Context, payload *model.ErrorsPayload) error {
	result := make(chan error, 1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.Errors <- SendErrorsRequest{payload, result}:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-result:
			return err
		}
	}
}

// SendMetrics sends a SendMetricsRequest value over the c.Metrics channel
// with the given payload, and waits for a response on the error channel
// included in the request, or for the context to be canceled.
func (c *ChannelTransport) SendMetrics(ctx context.Context, payload *model.MetricsPayload) error {
	result := make(chan error, 1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.Metrics <- SendMetricsRequest{payload, result}:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-result:
			return err
		}
	}
}
