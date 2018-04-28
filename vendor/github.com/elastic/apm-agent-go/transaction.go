package elasticapm

import (
	"sync"
	"time"

	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
)

// StartTransaction returns a new Transaction with the specified
// name and type, and with the start time set to the current time.
func (t *Tracer) StartTransaction(name, transactionType string) *Transaction {
	tx := t.newTransaction(name, transactionType)
	tx.Timestamp = time.Now()
	return tx
}

// newTransaction returns a new Transaction with the specified
// name and type, and sampling applied.
func (t *Tracer) newTransaction(name, transactionType string) *Transaction {
	tx, _ := t.transactionPool.Get().(*Transaction)
	if tx == nil {
		tx = &Transaction{tracer: t}
	}
	tx.model.Name = name
	tx.model.Type = transactionType

	// Take a snapshot of the max spans config to ensure
	// that once the maximum is reached, all future span
	// creations are dropped.
	t.maxSpansMu.RLock()
	tx.maxSpans = t.maxSpans
	t.maxSpansMu.RUnlock()

	t.samplerMu.RLock()
	sampler := t.sampler
	t.samplerMu.RUnlock()
	tx.sampled = true
	if sampler != nil && !sampler.Sample(tx) {
		tx.sampled = false
		tx.model.Sampled = &tx.sampled
	}
	return tx
}

// Transaction describes an event occurring in the monitored service.
//
// The ID, Spans, and SpanCount fields should not be modified
// directly. ID will be set by the Tracer when the transaction is
// flushed; the Span and SpanCount fields will be updated by the
// StartSpan method.
//
// Multiple goroutines should not attempt to update the Transaction
// fields concurrently, but may concurrently invoke any methods.
type Transaction struct {
	model     model.Transaction
	Timestamp time.Time
	Context   Context
	Result    string

	tracer   *Tracer
	sampled  bool
	maxSpans int

	mu    sync.Mutex
	spans []*Span
}

func (tx *Transaction) setID() {
	if tx.model.ID != "" {
		return
	}
	transactionID, err := NewUUID()
	if err != nil {
		// We ignore the error from NewUUID, which will
		// only occur if the entropy source fails. In
		// that case, there's nothing we can do. We don't
		// want to panic inside the user's application.
		return
	}
	tx.model.ID = transactionID
}

func (tx *Transaction) setContext(setter stacktrace.ContextSetter, pre, post int) error {
	for _, s := range tx.model.Spans {
		if err := stacktrace.SetContext(setter, s.Stacktrace, pre, post); err != nil {
			return err
		}
	}
	return nil
}

// reset resets the Transaction back to its zero state, so it can be reused
// in the transaction pool.
func (tx *Transaction) reset() {
	for _, s := range tx.spans {
		s.reset()
		tx.tracer.spanPool.Put(s)
	}
	*tx = Transaction{
		model: model.Transaction{
			Spans: tx.model.Spans[:0],
		},
		tracer:  tx.tracer,
		spans:   tx.spans[:0],
		Context: tx.Context,
	}
	tx.Context.reset()
}

// Sampled reports whether or not the transaction is sampled.
func (tx *Transaction) Sampled() bool {
	return tx.sampled
}

// Done sets the transaction's duration to the specified value, and
// enqueues it for sending to the Elastic APM server. The Transaction
// must not be used after this.
//
// If the duration specified is negative, then Done will set the
// duration to "time.Since(tx.Timestamp)" instead.
func (tx *Transaction) Done(d time.Duration) {
	if d < 0 {
		d = time.Since(tx.Timestamp)
	}
	tx.model.Duration = d.Seconds() * 1000
	tx.model.Timestamp = model.Time(tx.Timestamp.UTC())
	tx.model.Result = tx.Result

	tx.mu.Lock()
	spans := tx.spans[:len(tx.spans)]
	tx.mu.Unlock()
	if len(spans) != 0 {
		tx.model.Spans = make([]*model.Span, len(spans))
		for i, s := range spans {
			s.truncate(d)
			tx.model.Spans[i] = &s.model
		}
	}

	tx.enqueue()
}

func (tx *Transaction) enqueue() {
	select {
	case tx.tracer.transactions <- tx:
	default:
		// Enqueuing a transaction should never block.
		tx.tracer.statsMu.Lock()
		tx.tracer.stats.TransactionsDropped++
		tx.tracer.statsMu.Unlock()
		tx.reset()
		tx.tracer.transactionPool.Put(tx)
	}
}

// StartSpan starts and returns a new Span within the transaction,
// with the specified name, type, and optional parent span, and
// with the start time set to the current time relative to the
// transaction's timestamp. The span's ID will be set.
//
// If the transaction is not being sampled, then StartSpan will
// return nil.
//
// If the transaction is sampled, then the span's ID will be set,
// and its stacktrace will be set if the tracer is configured
// accordingly.
func (tx *Transaction) StartSpan(name, transactionType string, parent *Span) *Span {
	if !tx.Sampled() {
		return nil
	}

	start := time.Now()
	span, _ := tx.tracer.spanPool.Get().(*Span)
	if span == nil {
		span = &Span{}
	}
	span.tx = tx
	span.model.Name = name
	span.model.Type = transactionType
	span.Start = start

	tx.mu.Lock()
	if tx.maxSpans > 0 && len(tx.spans) >= tx.maxSpans {
		span.dropped = true
		tx.model.SpanCount.Dropped.Total++
	} else {
		if parent != nil {
			span.model.Parent = parent.model.ID
		}
		spanID := int64(len(tx.spans))
		span.model.ID = &spanID
		tx.spans = append(tx.spans, span)
	}
	tx.mu.Unlock()
	return span
}

// Span describes an operation within a transaction.
type Span struct {
	model   model.Span
	Start   time.Time
	Context SpanContext

	stacktrace []stacktrace.Frame
	tx         *Transaction
	dropped    bool

	mu        sync.Mutex
	done      bool
	truncated bool
}

func (s *Span) reset() {
	*s = Span{
		model: model.Span{
			Stacktrace: s.model.Stacktrace[:0],
		},
		Context:    s.Context,
		stacktrace: s.stacktrace[:0],
	}
	s.Context.reset()
}

// SetStacktrace sets the stacktrace for the span,
// skipping the first skip number of frames,
// excluding the SetStacktrace function.
//
// If the span is dropped, this method is a no-op.
func (s *Span) SetStacktrace(skip int) {
	if s.Dropped() {
		return
	}
	s.stacktrace = stacktrace.AppendStacktrace(s.stacktrace[:0], skip+1, -1)
	s.model.Stacktrace = appendModelStacktraceFrames(s.model.Stacktrace[:0], s.stacktrace)
}

// Dropped indicates whether or not the span is dropped, meaning it
// will not be included in the transaction. Spans are dropped when
// the configurable limit is reached.
func (s *Span) Dropped() bool {
	return s.dropped
}

// Done sets the span's duration to the specified value. The Span
// must not be used after this.
//
// If the duration specified is negative, then Done will set the
// duration to "time.Since(s.Start)" instead.
//
// If the span is dropped, this method is a no-op.
func (s *Span) Done(d time.Duration) {
	if s.Dropped() {
		return
	}
	if d < 0 {
		d = time.Since(s.Start)
	}
	s.mu.Lock()
	s.model.Start = s.Start.Sub(s.tx.Timestamp).Seconds() * 1000
	s.model.Context = s.Context.build()
	if !s.truncated {
		s.done = true
		s.model.Duration = d.Seconds() * 1000
	}
	s.mu.Unlock()
}

func (s *Span) truncate(d time.Duration) {
	s.mu.Lock()
	if !s.done {
		s.truncated = true
		s.model.Type += ".truncated"
		s.model.Duration = d.Seconds() * 1000
	}
	s.mu.Unlock()
}
