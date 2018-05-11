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
func (t *Tracer) StartTransaction(name, transactionType string, opts ...TransactionOption) *Transaction {
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

	var txOpts transactionOptions
	for _, o := range opts {
		o(&txOpts)
	}

	// Generate a random transaction ID.
	binary.LittleEndian.PutUint64(tx.id[:8], tx.rand.Uint64())
	binary.LittleEndian.PutUint64(tx.id[8:], tx.rand.Uint64())

	// Take a snapshot of the max spans config to ensure
	// that once the maximum is reached, all future span
	// creations are dropped.
	t.maxSpansMu.RLock()
	tx.maxSpans = t.maxSpans
	t.maxSpansMu.RUnlock()

	t.spanFramesMinDurationMu.RLock()
	tx.spanFramesMinDuration = t.spanFramesMinDuration
	t.spanFramesMinDurationMu.RUnlock()

	t.samplerMu.RLock()
	sampler := t.sampler
	t.samplerMu.RUnlock()
	tx.sampled = true
	if sampler != nil && !sampler.Sample(tx) {
		tx.sampled = false
	}
	tx.Timestamp = time.Now()
	return tx
}

// Transaction describes an event occurring in the monitored service.
type Transaction struct {
	Name      string
	Type      string
	Timestamp time.Time
	Duration  time.Duration
	Context   Context
	Result    string
	id        [16]byte

	tracer                *Tracer
	sampled               bool
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
	return tx.sampled
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

// TransactionOption sets options when starting a transaction.
type TransactionOption func(*transactionOptions)

type transactionOptions struct{}
