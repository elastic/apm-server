package transporttest

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/internal/fastjson"
	"github.com/elastic/apm-agent-go/model"
)

// NewRecorderTracer returns a new elasticapm.Tracer and
// RecorderTransport, which is set as the tracer's transport.
func NewRecorderTracer() (*elasticapm.Tracer, *RecorderTransport) {
	var transport RecorderTransport
	tracer, err := elasticapm.NewTracer("transporttest", "")
	if err != nil {
		panic(err)
	}
	tracer.Transport = &transport
	return tracer, &transport
}

// RecorderTransport implements transport.Transport,
// recording the payloads sent. The payloads can be
// retrieved using the Payloads method.
type RecorderTransport struct {
	mu       sync.Mutex
	payloads Payloads
}

// SendTransactions records the transactions payload such that it can later be
// obtained via Payloads.
func (r *RecorderTransport) SendTransactions(ctx context.Context, payload *model.TransactionsPayload) error {
	return r.record(payload, &model.TransactionsPayload{})
}

// SendErrors records the errors payload such that it can later be obtained via
// Payloads.
func (r *RecorderTransport) SendErrors(ctx context.Context, payload *model.ErrorsPayload) error {
	return r.record(payload, &model.ErrorsPayload{})
}

// SendMetrics records the metrics payload such that it can later be obtained via
// Payloads.
func (r *RecorderTransport) SendMetrics(ctx context.Context, payload *model.MetricsPayload) error {
	return r.record(payload, &model.MetricsPayload{})
}

// Payloads returns the payloads recorded by SendTransactions and SendErrors.
// Each element of Payloads is a deep copy of the *model.TransactionsPayload
// or *model.ErrorsPayload, produced by encoding/decoding the payload to/from
// JSON.
func (r *RecorderTransport) Payloads() Payloads {
	r.mu.Lock()
	payloads := r.payloads[:]
	r.mu.Unlock()
	return payloads
}

func (r *RecorderTransport) record(payload, zeroPayload interface{}) error {
	var w fastjson.Writer
	fastjson.Marshal(&w, payload)
	if err := json.Unmarshal(w.Bytes(), zeroPayload); err != nil {
		panic(err)
	}
	r.mu.Lock()
	r.payloads = append(r.payloads, Payload{zeroPayload})
	r.mu.Unlock()
	return nil
}

// Payloads is a slice of Payload.
type Payloads []Payload

// Payload wraps an untyped payload value. Use the methods to obtain the
// appropriate payload type fields.
type Payload struct {
	Value interface{}
}

// Transactions returns the transactions within the payload. If the payload
// is not a transactions payload, this will panic.
func (p Payload) Transactions() []*model.Transaction {
	return p.Value.(*model.TransactionsPayload).Transactions
}

// Errors returns the errors within the payload. If the payload
// is not an errors payload, this will panic.
func (p Payload) Errors() []*model.Error {
	return p.Value.(*model.ErrorsPayload).Errors
}

// Metrics returns the metrics within the payload. If the payload
// is not a metrics payload, this will panic.
func (p Payload) Metrics() []*model.Metrics {
	return p.Value.(*model.MetricsPayload).Metrics
}
