package apm

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
	td, _ := t.transactionDataPool.Get().(*TransactionData)
	if td == nil {
		td = &TransactionData{
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
		td.rand = rand.New(rand.NewSource(seed))
	}
	tx := &Transaction{TransactionData: td}

	tx.Name = name
	tx.Type = transactionType

	var root bool
	if opts.TraceContext.Trace.Validate() == nil && opts.TraceContext.Span.Validate() == nil {
		tx.traceContext.Trace = opts.TraceContext.Trace
		tx.parentSpan = opts.TraceContext.Span
		binary.LittleEndian.PutUint64(tx.traceContext.Span[:], tx.rand.Uint64())
	} else {
		// Start a new trace. We reuse the trace ID for the root transaction's ID.
		root = true
		binary.LittleEndian.PutUint64(tx.traceContext.Trace[:8], tx.rand.Uint64())
		binary.LittleEndian.PutUint64(tx.traceContext.Trace[8:], tx.rand.Uint64())
		copy(tx.traceContext.Span[:], tx.traceContext.Trace[:])
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

	if root {
		t.samplerMu.RLock()
		sampler := t.sampler
		t.samplerMu.RUnlock()
		if sampler == nil || sampler.Sample(tx.traceContext) {
			o := tx.traceContext.Options.WithRecorded(true)
			tx.traceContext.Options = o
		}
	} else {
		// TODO(axw) make this behaviour configurable. In some cases
		// it may not be a good idea to honour the recorded flag, as
		// it may open up the application to DoS by forced sampling.
		// Even ignoring bad actors, a service that has many feeder
		// applications may end up being sampled at a very high rate.
		tx.traceContext.Options = opts.TraceContext.Options
	}
	tx.timestamp = opts.Start
	if tx.timestamp.IsZero() {
		tx.timestamp = time.Now()
	}
	return tx
}

// TransactionOptions holds options for Tracer.StartTransactionOptions.
type TransactionOptions struct {
	// TraceContext holds the TraceContext for a new transaction. If this is
	// zero, a new trace will be started.
	TraceContext TraceContext

	// Start is the start time of the transaction. If this has the
	// zero value, time.Now() will be used instead.
	Start time.Time
}

// Transaction describes an event occurring in the monitored service.
type Transaction struct {
	mu sync.RWMutex

	// TransactionData holds the transaction data. This field is set to
	// nil when either of the transaction's End or Discard methods are called.
	*TransactionData
}

// Sampled reports whether or not the transaction is sampled.
func (tx *Transaction) Sampled() bool {
	if tx == nil {
		return false
	}
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	if tx.ended() {
		return false
	}
	return tx.traceContext.Options.Recorded()
}

// TraceContext returns the transaction's TraceContext.
//
// The resulting TraceContext's Span field holds the transaction's ID.
// If tx is nil, or has already been ended, a zero (invalid) TraceContext
// is returned.
func (tx *Transaction) TraceContext() TraceContext {
	if tx == nil {
		return TraceContext{}
	}
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	if tx.ended() {
		return TraceContext{}
	}
	return tx.traceContext
}

// EnsureParent returns the span ID for for tx's parent, generating a
// parent span ID if one has not already been set. If tx is nil, a zero
// (invalid) SpanID is returned.
//
// This method can be used for generating a span ID for the RUM
// (Real User Monitoring) agent, where the RUM agent is initialized
// after the backend service returns.
func (tx *Transaction) EnsureParent() SpanID {
	if tx == nil {
		return SpanID{}
	}
	tx.TransactionData.mu.Lock()
	if tx.parentSpan.isZero() {
		// parentSpan can only be zero if tx is a root transaction
		// for which GenerateParentTraceContext() has not previously
		// been called. Reuse the latter half of the trace ID for
		// the parent span ID; the first half is used for the
		// transaction ID.
		copy(tx.parentSpan[:], tx.traceContext.Trace[8:])
	}
	tx.TransactionData.mu.Unlock()
	return tx.parentSpan
}

// Discard discards a previously started transaction.
//
// Calling Discard will set tx's TransactionData field to nil, so callers must
// ensure tx is not updated after Discard returns.
func (tx *Transaction) Discard() {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.ended() {
		return
	}
	tx.TransactionData.reset()
	tx.TransactionData = nil
}

// End enqueues tx for sending to the Elastic APM server.
//
// Calling End will set tx's TransactionData field to nil, so callers
// must ensure tx is not updated after End returns.
//
// If tx.Duration has not been set, End will set it to the elapsed time
// since the transaction's start time.
func (tx *Transaction) End() {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.ended() {
		return
	}
	if tx.Duration < 0 {
		tx.Duration = time.Since(tx.timestamp)
	}
	tx.TransactionData.enqueue()
	tx.TransactionData = nil
}

// ended reports whether or not End or Discard has been called.
//
// This must be called with tx.mu held for reading.
func (tx *Transaction) ended() bool {
	return tx.TransactionData == nil
}

// TransactionData holds the details for a transaction, and is embedded
// inside Transaction. When a transaction is ended, its TransactionData
// field will be set to nil.
type TransactionData struct {
	// Name holds the transaction name, initialized with the value
	// passed to StartTransaction.
	Name string

	// Type holds the transaction type, initialized with the value
	// passed to StartTransaction.
	Type string

	// Duration holds the transaction duration, initialized to -1.
	//
	// If you do not update Duration, calling Transaction.End will
	// calculate the duration based on the elapsed time since the
	// transaction's start time.
	Duration time.Duration

	// Context describes the context in which the transaction occurs.
	Context Context

	// Result holds the transaction result.
	Result string

	tracer                *Tracer
	maxSpans              int
	spanFramesMinDuration time.Duration
	timestamp             time.Time
	traceContext          TraceContext

	mu           sync.Mutex
	parentSpan   SpanID
	spansCreated int
	spansDropped int
	rand         *rand.Rand // for ID generation
}

func (td *TransactionData) enqueue() {
	select {
	case td.tracer.transactions <- td:
	default:
		// Enqueuing a transaction should never block.
		td.tracer.statsMu.Lock()
		td.tracer.stats.TransactionsDropped++
		td.tracer.statsMu.Unlock()
		td.reset()
	}
}

// reset resets the Transaction back to its zero state and places it back
// into the transaction pool.
func (td *TransactionData) reset() {
	*td = TransactionData{
		tracer:   td.tracer,
		Context:  td.Context,
		Duration: -1,
		rand:     td.rand,
	}
	td.Context.reset()
	td.tracer.transactionDataPool.Put(td)
}
