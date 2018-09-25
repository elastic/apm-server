package elasticapm

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync"
	"time"
)

// StartTransaction returns a new Transaction with the specified
// name and type, and with the start time set to the current time.
// This is equivalent to calling StartTransactionOptions with a
// zero TransactionOptions.
func (t *Tracer) StartTransaction(name, transactionType string) *Transaction {
	return t.StartTransactionOptions(name, transactionType, TransactionOptions{})
}

// StartTransactionOptions returns a new Transaction with the
// specified name, type, and options.
func (t *Tracer) StartTransactionOptions(name, transactionType string, opts TransactionOptions) *Transaction {
	tx, _ := t.transactionPool.Get().(*Transaction)
	if tx == nil {
		tx = &Transaction{
			tracer:   t,
			Duration: -1,
			Context: Context{
				captureBodyMask: CaptureBodyTransactions,
			},
		}
		var seed int64
		if err := binary.Read(cryptorand.Reader, binary.LittleEndian, &seed); err != nil {
			seed = time.Now().UnixNano()
		}
		tx.rand = rand.New(rand.NewSource(seed))
	}
	tx.Name = name
	tx.Type = transactionType

	if tx.tracer.distributedTracing {
		if opts.TraceContext.Trace.Validate() == nil && opts.TraceContext.Span.Validate() == nil {
			tx.traceContext.Trace = opts.TraceContext.Trace
			tx.parentSpan = opts.TraceContext.Span
		}
		tx.traceContext.Options = opts.TraceContext.Options
	}
	if tx.traceContext.Trace.isZero() {
		// In any case, we fill out the trace ID; this will be used for
		// either the transaction's trace ID, or for its UUID in case
		// distributed tracing is disabled. If distributed tracing is
		// enabled, we also set the span ID.
		binary.LittleEndian.PutUint64(tx.traceContext.Trace[:8], tx.rand.Uint64())
		binary.LittleEndian.PutUint64(tx.traceContext.Trace[8:], tx.rand.Uint64())
		if tx.tracer.distributedTracing {
			copy(tx.traceContext.Span[:], tx.traceContext.Trace[:])
		}
	} else {
		binary.LittleEndian.PutUint64(tx.traceContext.Span[:], tx.rand.Uint64())
	}

	// Take a snapshot of the max spans config to ensure
	// that once the maximum is reached, all future span
	// creations are dropped.
	t.maxSpansMu.RLock()
	tx.maxSpans = t.maxSpans
	t.maxSpansMu.RUnlock()

	t.spanFramesMinDurationMu.RLock()
	tx.spanFramesMinDuration = t.spanFramesMinDuration
	t.spanFramesMinDurationMu.RUnlock()

	// TODO(axw) make this behaviour configurable. In some cases
	// it may not be a good idea to honour the sampled flag, as
	// it may open up the application to DoS by forced sampling.
	// Even ignoring bad actors, a service that has many feeder
	// applications may end up being sampled at a very high rate.
	if !tx.traceContext.Options.Requested() {
		t.samplerMu.RLock()
		sampler := t.sampler
		t.samplerMu.RUnlock()
		if sampler == nil || sampler.Sample(tx) {
			o := tx.traceContext.Options.WithRequested(true).WithMaybeRecorded(true)
			tx.traceContext.Options = o
		}
	}
	tx.Timestamp = opts.Start
	if tx.Timestamp.IsZero() {
		tx.Timestamp = time.Now()
	}
	return tx
}

// Transaction describes an event occurring in the monitored service.
type Transaction struct {
	Name         string
	Type         string
	Timestamp    time.Time
	Duration     time.Duration
	Context      Context
	Result       string
	traceContext TraceContext
	parentSpan   SpanID

	tracer                *Tracer
	maxSpans              int
	spanFramesMinDuration time.Duration

	mu           sync.Mutex
	spans        []*Span
	spansDropped int
	rand         *rand.Rand // for ID generation
}

// reset resets the Transaction back to its zero state, so it can be reused
// in the transaction pool.
func (tx *Transaction) reset() {
	for _, s := range tx.spans {
		s.reset()
		tx.tracer.spanPool.Put(s)
	}
	*tx = Transaction{
		tracer:   tx.tracer,
		spans:    tx.spans[:0],
		Context:  tx.Context,
		Duration: -1,
		rand:     tx.rand,
	}
	tx.Context.reset()
}

// Discard discards a previously started transaction. The Transaction
// must not be used after this.
func (tx *Transaction) Discard() {
	tx.reset()
	tx.tracer.transactionPool.Put(tx)
}

// Sampled reports whether or not the transaction is sampled.
func (tx *Transaction) Sampled() bool {
	return tx.traceContext.Options.MaybeRecorded()
}

// TraceContext returns the transaction's TraceContext: its trace ID,
// span ID, and trace options. The values are undefined if distributed
// tracing is disabled.
func (tx *Transaction) TraceContext() TraceContext {
	return tx.traceContext
}

// End enqueues tx for sending to the Elastic APM server; tx must not
// be used after this.
//
// If tx.Duration has not been set, End will set it to the elapsed
// time since tx.Timestamp.
func (tx *Transaction) End() {
	if tx.Duration < 0 {
		tx.Duration = time.Since(tx.Timestamp)
	}
	for _, s := range tx.spans {
		s.finalize(tx.Timestamp.Add(tx.Duration))
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

// TransactionOptions holds options for Tracer.StartTransactionOptions.
type TransactionOptions struct {
	// TraceContext holds the TraceContext for a new transaction. If this is
	// zero, and distributed tracing is enabled, a new trace will be started.
	//
	// TraceContext is ignored if distributed tracing is disabled.
	TraceContext TraceContext

	// Start is the start time of the transaction. If this has the
	// zero value, time.Now() will be used instead.
	Start time.Time
}
