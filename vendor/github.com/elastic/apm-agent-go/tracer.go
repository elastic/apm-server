package elasticapm

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
	"github.com/elastic/apm-agent-go/transport"
)

const (
	defaultPreContext      = 3
	defaultPostContext     = 3
	transactionsChannelCap = 1000
	errorsChannelCap       = 1000

	// defaultMaxErrorQueueSize is the default maximum number
	// of errors to enqueue in the tracer. When this fills up,
	// errors will start being dropped (when the channel is
	// also full).
	defaultMaxErrorQueueSize = 1000

	// TODO(axw) make this configurable
	metricsInterval = 5 * time.Second
)

var (
	// DefaultTracer is the default global Tracer, set at package
	// initialization time, configured via environment variables.
	//
	// This will always be initialized to a non-nil value. If any
	// of the environment variables are invalid, the corresponding
	// errors will be logged to stderr and the default values will
	// be used instead.
	DefaultTracer *Tracer
)

func init() {
	var opts options
	opts.init(true)
	DefaultTracer = newTracer(&envService, opts)
}

type options struct {
	flushInterval           time.Duration
	maxTransactionQueueSize int
	maxSpans                int
	sampler                 Sampler
}

func (opts *options) init(continueOnError bool) error {
	var errs []error
	flushInterval, err := initialFlushInterval()
	if err != nil {
		flushInterval = defaultFlushInterval
		errs = append(errs, err)
	}
	maxTransactionQueueSize, err := initialMaxTransactionQueueSize()
	if err != nil {
		maxTransactionQueueSize = defaultMaxTransactionQueueSize
		errs = append(errs, err)
	}
	maxSpans, err := initialMaxSpans()
	if err != nil {
		maxSpans = defaultMaxSpans
		errs = append(errs, err)
	}
	sampler, err := initialSampler()
	if err != nil {
		sampler = nil
		errs = append(errs, err)
	}
	if len(errs) != 0 && !continueOnError {
		return errs[0]
	}
	for _, err := range errs {
		log.Printf("[elasticapm]: %s", err)
	}
	opts.flushInterval = flushInterval
	opts.maxTransactionQueueSize = maxTransactionQueueSize
	opts.maxSpans = maxSpans
	opts.sampler = sampler
	return nil
}

// Tracer manages the sampling and sending of transactions to
// Elastic APM.
//
// Transactions are buffered until they are flushed (forcibly
// with a Flush call, or when the flush timer expires), or when
// the maximum transaction queue size is reached. Failure to
// send will be periodically retried. Once the queue limit has
// been reached, new transactions will replace older ones in
// the queue.
//
// Errors are sent as soon as possible, but will buffered and
// later sent in bulk if the tracer is busy, or otherwise cannot
// send to the server, e.g. due to network failure. There is
// a limit to the number of errors that will be buffered, and
// once that limit has been reached, new errors will be dropped
// until the queue is drained.
//
// The exported fields be altered or replaced any time up until
// any Tracer methods have been invoked.
type Tracer struct {
	Transport transport.Transport
	Service   *model.Service

	process *model.Process
	system  *model.System

	closing                    chan struct{}
	closed                     chan struct{}
	forceFlush                 chan chan<- struct{}
	sendMetrics                chan chan<- struct{}
	addMetricsGatherer         chan MetricsGatherer
	setFlushInterval           chan time.Duration
	setMaxTransactionQueueSize chan int
	setMaxErrorQueueSize       chan int
	setPreContext              chan int
	setPostContext             chan int
	setContextSetter           chan stacktrace.ContextSetter
	setLogger                  chan Logger
	setProcessor               chan Processor
	transactions               chan *Transaction
	errors                     chan *Error

	statsMu sync.Mutex
	stats   TracerStats

	maxSpansMu sync.RWMutex
	maxSpans   int

	samplerMu sync.RWMutex
	sampler   Sampler

	errorPool       sync.Pool
	spanPool        sync.Pool
	transactionPool sync.Pool
}

// NewTracer returns a new Tracer, using the default transport,
// initializing a Service with the specified name and version,
// or taking the service name and version from the environment
// if unspecified.
//
// If serviceName is empty, then the service name will be defined
// using the ELASTIC_APM_SERVER_NAME environment variable.
func NewTracer(serviceName, serviceVersion string) (*Tracer, error) {
	service := &envService
	if serviceName != "" {
		if err := validateServiceName(serviceName); err != nil {
			return nil, err
		}
		service = newService(serviceName, serviceVersion)
	}
	var opts options
	if err := opts.init(false); err != nil {
		return nil, err
	}
	return newTracer(service, opts), nil
}

func newTracer(service *model.Service, opts options) *Tracer {
	t := &Tracer{
		Transport:                  transport.Default,
		Service:                    service,
		process:                    &currentProcess,
		system:                     &localSystem,
		closing:                    make(chan struct{}),
		closed:                     make(chan struct{}),
		forceFlush:                 make(chan chan<- struct{}),
		sendMetrics:                make(chan chan<- struct{}),
		addMetricsGatherer:         make(chan MetricsGatherer),
		setFlushInterval:           make(chan time.Duration),
		setMaxTransactionQueueSize: make(chan int),
		setMaxErrorQueueSize:       make(chan int),
		setPreContext:              make(chan int),
		setPostContext:             make(chan int),
		setContextSetter:           make(chan stacktrace.ContextSetter),
		setLogger:                  make(chan Logger),
		setProcessor:               make(chan Processor),
		transactions:               make(chan *Transaction, transactionsChannelCap),
		errors:                     make(chan *Error, errorsChannelCap),
		maxSpans:                   opts.maxSpans,
		sampler:                    opts.sampler,
	}
	go t.loop()
	t.setFlushInterval <- opts.flushInterval
	t.setMaxTransactionQueueSize <- opts.maxTransactionQueueSize
	t.setMaxErrorQueueSize <- defaultMaxErrorQueueSize
	t.setPreContext <- defaultPreContext
	t.setPostContext <- defaultPostContext
	return t
}

// Close closes the Tracer, preventing transactions from being
// sent to the APM server.
func (t *Tracer) Close() {
	select {
	case <-t.closing:
	default:
		close(t.closing)
	}
	<-t.closed
}

// Flush waits for the Tracer to flush any transactions and errors it currently
// has queued to the APM server, the tracer is stopped, or the abort channel
// is signaled.
func (t *Tracer) Flush(abort <-chan struct{}) {
	flushed := make(chan struct{}, 1)
	select {
	case t.forceFlush <- flushed:
		select {
		case <-abort:
		case <-flushed:
		case <-t.closed:
		}
	case <-t.closed:
	}
}

// SetFlushInterval sets the flush interval -- the amount of time
// to wait before flushing enqueued transactions to the APM server.
func (t *Tracer) SetFlushInterval(d time.Duration) {
	select {
	case t.setFlushInterval <- d:
	case <-t.closing:
	case <-t.closed:
	}
}

// SetMaxTransactionQueueSize sets the maximum transaction queue size -- the
// maximum number of transactions to buffer before flushing to the APM server.
// If set to a non-positive value, the queue size is unlimited.
func (t *Tracer) SetMaxTransactionQueueSize(n int) {
	select {
	case t.setMaxTransactionQueueSize <- n:
	case <-t.closing:
	case <-t.closed:
	}
}

// SetMaxErrorQueueSize sets the maximum error queue size -- the
// maximum number of errors to buffer before they will start getting
// dropped. If set to a non-positive value, the queue size is unlimited.
func (t *Tracer) SetMaxErrorQueueSize(n int) {
	select {
	case t.setMaxErrorQueueSize <- n:
	case <-t.closing:
	case <-t.closed:
	}
}

// SetContextSetter sets the stacktrace.ContextSetter to be used for
// setting stacktrace source context. If nil (which is the initial
// value), no context will be set.
func (t *Tracer) SetContextSetter(setter stacktrace.ContextSetter) {
	select {
	case t.setContextSetter <- setter:
	case <-t.closing:
	case <-t.closed:
	}
}

// SetLogger sets the Logger to be used for logging the operation of
// the tracer.
func (t *Tracer) SetLogger(logger Logger) {
	select {
	case t.setLogger <- logger:
	case <-t.closing:
	case <-t.closed:
	}
}

// SetProcessor sets the processors for the tracer.
func (t *Tracer) SetProcessor(p ...Processor) {
	var processor Processor
	if len(p) > 0 {
		processor = Processors(p)
	}
	select {
	case t.setProcessor <- processor:
	case <-t.closing:
	case <-t.closed:
	}
}

// SetSampler sets the sampler the tracer. It is valid to pass nil,
// in which case all transactions will be sampled.
func (t *Tracer) SetSampler(s Sampler) {
	t.samplerMu.Lock()
	t.sampler = s
	t.samplerMu.Unlock()
}

// SetMaxSpans sets the maximum number of spans that will be added
// to a transaction before dropping. If set to a non-positive value,
// the number of spans is unlimited.
func (t *Tracer) SetMaxSpans(n int) {
	t.maxSpansMu.Lock()
	t.maxSpans = n
	t.maxSpansMu.Unlock()
}

// AddMetricsGatherer schedules g to gather metrics periodically.
//
// TODO(axw) return a function that can be used to remove the gatherer.
func (t *Tracer) AddMetricsGatherer(g MetricsGatherer) {
	select {
	case t.addMetricsGatherer <- g:
	case <-t.closing:
	case <-t.closed:
	}
}

func (t *Tracer) SendMetrics(abort <-chan struct{}) {
	sent := make(chan struct{}, 1)
	select {
	case t.sendMetrics <- sent:
		select {
		case <-abort:
		case <-sent:
		case <-t.closed:
		}
	case <-t.closed:
	}
}

// Stats returns the current TracerStats. This will return the most
// recent values even after the tracer has been closed.
func (t *Tracer) Stats() TracerStats {
	t.statsMu.Lock()
	stats := t.stats
	t.statsMu.Unlock()
	return stats
}

func (t *Tracer) loop() {
	defer close(t.closed)

	ctx, cancelContext := context.WithCancel(context.Background())
	defer cancelContext()
	go func() {
		select {
		case <-t.closing:
			cancelContext()
		}
	}()

	var metrics Metrics
	var metricsGatherers []metricsGatherer
	var flushInterval time.Duration
	var flushed chan<- struct{}
	var sentMetrics chan<- struct{}
	var maxTransactionQueueSize int
	var maxErrorQueueSize int
	var flushC <-chan time.Time
	var sendMetricsC <-chan time.Time
	var transactions []*Transaction
	var errors []*Error
	var statsUpdates TracerStats
	sender := sender{
		tracer: t,
		stats:  &statsUpdates,
	}

	errorsC := t.errors
	forceFlush := t.forceFlush
	forceSendMetrics := t.sendMetrics
	flushTimer := time.NewTimer(0)
	if !flushTimer.Stop() {
		<-flushTimer.C
	}
	metricsTimer := time.NewTimer(0)
	if !metricsTimer.Stop() {
		<-metricsTimer.C
	}
	startTimer := func(ch *<-chan time.Time, timer *time.Timer, interval time.Duration) {
		if *ch != nil {
			// Timer already started.
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(interval)
		*ch = timer.C
	}
	startFlushTimer := func() {
		startTimer(&flushC, flushTimer, flushInterval)
	}
	startMetricsTimer := func() {
		startTimer(&sendMetricsC, metricsTimer, metricsInterval)
	}

	receivedTransaction := func(tx *Transaction, stats *TracerStats) {
		if maxTransactionQueueSize > 0 && len(transactions) >= maxTransactionQueueSize {
			// The queue is full, so pop the oldest item.
			// TODO(axw) use container/ring? implement
			// ring buffer on top of slice? profile
			n := uint64(len(transactions) - maxTransactionQueueSize + 1)
			for _, tx := range transactions[:n] {
				tx.reset()
				t.transactionPool.Put(tx)
			}
			transactions = transactions[n:]
			stats.TransactionsDropped += n
		}
		transactions = append(transactions, tx)
	}

	for {
		var sendMetrics bool
		var sendTransactions bool
		statsUpdates = TracerStats{}

		select {
		case <-t.closing:
			return
		case g := <-t.addMetricsGatherer:
			metricsGatherers = append(metricsGatherers, metricsGatherer{
				MetricsGatherer: g,
			})
			if len(metricsGatherers) == 1 {
				// First metrics gatherer added, start timer.
				startMetricsTimer()
			}
			continue
		case flushInterval = <-t.setFlushInterval:
			continue
		case maxTransactionQueueSize = <-t.setMaxTransactionQueueSize:
			if maxTransactionQueueSize <= 0 || len(transactions) < maxTransactionQueueSize {
				continue
			}
		case maxErrorQueueSize = <-t.setMaxErrorQueueSize:
			if maxErrorQueueSize <= 0 || len(errors) < maxErrorQueueSize {
				errorsC = t.errors
			}
			continue
		case sender.preContext = <-t.setPreContext:
			continue
		case sender.postContext = <-t.setPostContext:
			continue
		case sender.contextSetter = <-t.setContextSetter:
			continue
		case sender.logger = <-t.setLogger:
			continue
		case sender.processor = <-t.setProcessor:
			continue
		case e := <-errorsC:
			errors = append(errors, e)
		case tx := <-t.transactions:
			beforeLen := len(transactions)
			receivedTransaction(tx, &statsUpdates)
			if len(transactions) == beforeLen && flushC != nil {
				// The queue was already full, and a retry
				// timer is running; wait for it to fire.
				t.statsMu.Lock()
				t.stats.accumulate(statsUpdates)
				t.statsMu.Unlock()
				continue
			}
			if maxTransactionQueueSize <= 0 || len(transactions) < maxTransactionQueueSize {
				startFlushTimer()
				continue
			}
			sendTransactions = true
		case <-flushC:
			flushC = nil
			sendTransactions = true
		case flushed = <-forceFlush:
			// The caller has explicitly requested a flush, so
			// drain any transactions buffered in the channel.
			for n := len(t.transactions); n > 0; n-- {
				tx := <-t.transactions
				receivedTransaction(tx, &statsUpdates)
			}
			// flushed will be signaled, and forceFlush set back to
			// t.forceFlush, when the queued transactions and/or
			// errors are successfully sent.
			forceFlush = nil
			flushC = nil
			sendTransactions = true
		case <-sendMetricsC:
			sendMetricsC = nil
			sendMetrics = true
		case sentMetrics = <-forceSendMetrics:
			// sentMetrics will be signaled, and forceSendMetrics
			// set back to t.sendMetrics, when metrics have been
			// gathered and an attempt to send them has been made.
			forceSendMetrics = nil
			sendMetricsC = nil
			sendMetrics = true
		}

		if remainder := maxErrorQueueSize - len(errors); remainder > 0 {
			// Drain any errors in the channel, up to the maximum queue size.
			for n := len(t.errors); n > 0 && remainder > 0; n-- {
				errors = append(errors, <-t.errors)
				remainder--
			}
		}
		if sender.sendErrors(ctx, errors) {
			for _, e := range errors {
				e.reset()
				t.errorPool.Put(e)
			}
			errors = errors[:0]
			errorsC = t.errors
		} else if len(errors) == maxErrorQueueSize {
			errorsC = nil
		}
		if sendTransactions {
			if sender.sendTransactions(ctx, transactions) {
				for _, tx := range transactions {
					tx.reset()
					t.transactionPool.Put(tx)
				}
				transactions = transactions[:0]
			}
		}
		if !statsUpdates.isZero() {
			t.statsMu.Lock()
			t.stats.accumulate(statsUpdates)
			t.statsMu.Unlock()
		}

		if sendMetrics {
			// TODO(axw) always report tracer stats or introduce
			// a gatherer for gathering the tracer stats?
			//
			// TODO(axw) gather metrics in parallel with sending
			// transactions and errors. Kick off all goroutines,
			// have the last one to complete signal a channel
			// which will tell the tracer it's time to send.
			for _, g := range metricsGatherers {
				if err := g.gather(ctx, &metrics); err != nil {
					if sender.logger != nil {
						sender.logger.Debugf("gathering metrics failed: %s", err)
					}
				}
			}
			timestamp := model.Time(time.Now())
			for _, s := range metrics.metrics {
				s.Timestamp = timestamp
			}
			sender.sendMetrics(ctx, &metrics)
			metrics.reset()
			if sentMetrics != nil {
				sentMetrics <- struct{}{}
				sentMetrics = nil
			}
			startMetricsTimer()
		}

		if statsUpdates.Errors.SendTransactions != 0 || statsUpdates.Errors.SendErrors != 0 {
			// Sending transactions or errors failed, start a new timer to resend.
			startFlushTimer()
			continue
		}
		if sendTransactions && flushed != nil {
			forceFlush = t.forceFlush
			flushed <- struct{}{}
			flushed = nil
		}
	}
}

type sender struct {
	tracer                  *Tracer
	logger                  Logger
	processor               Processor
	contextSetter           stacktrace.ContextSetter
	preContext, postContext int
	stats                   *TracerStats
}

// sendTransactions attempts to send enqueued transactions to the APM server,
// returning true if the transactions were successfully sent.
func (s *sender) sendTransactions(ctx context.Context, transactions []*Transaction) bool {
	if len(transactions) == 0 {
		return false
	}
	if s.contextSetter != nil {
		var err error
		for _, tx := range transactions {
			if err = tx.setContext(s.contextSetter, s.preContext, s.postContext); err != nil {
				break
			}
		}
		if err != nil {
			if s.logger != nil {
				s.logger.Debugf("setting context failed: %s", err)
			}
			s.stats.Errors.SetContext++
		}
	}
	payload := model.TransactionsPayload{
		Service:      s.tracer.Service,
		Process:      s.tracer.process,
		System:       s.tracer.system,
		Transactions: make([]*model.Transaction, len(transactions)),
	}
	for i, tx := range transactions {
		tx.setID()
		if tx.Sampled() {
			tx.model.Context = tx.Context.build()
		}
		if s.processor != nil {
			s.processor.ProcessTransaction(&tx.model)
		}
		payload.Transactions[i] = &tx.model
	}
	if err := s.tracer.Transport.SendTransactions(ctx, &payload); err != nil {
		if s.logger != nil {
			s.logger.Debugf("sending transactions failed: %s", err)
		}
		s.stats.Errors.SendTransactions++
		return false
	}
	s.stats.TransactionsSent += uint64(len(transactions))
	return true
}

// sendErrors attempts to send enqueued errors to the APM server,
// returning true if the errors were successfully sent.
func (s *sender) sendErrors(ctx context.Context, errors []*Error) bool {
	if len(errors) == 0 {
		return false
	}
	if s.contextSetter != nil {
		var err error
		for _, e := range errors {
			if err = e.setContext(s.contextSetter, s.preContext, s.postContext); err != nil {
				break
			}
		}
		if err != nil {
			if s.logger != nil {
				s.logger.Debugf("setting context failed: %s", err)
			}
			s.stats.Errors.SetContext++
		}
	}
	payload := model.ErrorsPayload{
		Service: s.tracer.Service,
		Process: s.tracer.process,
		System:  s.tracer.system,
		Errors:  make([]*model.Error, len(errors)),
	}
	for i, e := range errors {
		if e.Transaction != nil {
			e.Transaction.setID()
			e.model.Transaction.ID = e.Transaction.model.ID
		}
		e.setStacktrace()
		e.setCulprit()
		e.model.ID = e.ID
		e.model.Timestamp = model.Time(e.Timestamp.UTC())
		e.model.Context = e.Context.build()
		e.model.Exception.Handled = e.Handled
		if s.processor != nil {
			s.processor.ProcessError(&e.model)
		}
		payload.Errors[i] = &e.model
	}
	if err := s.tracer.Transport.SendErrors(ctx, &payload); err != nil {
		if s.logger != nil {
			s.logger.Debugf("sending errors failed: %s", err)
		}
		s.stats.Errors.SendErrors++
		return false
	}
	s.stats.ErrorsSent += uint64(len(errors))
	return true
}

// sendMetrics attempts to send metrics to the APM server.
func (s *sender) sendMetrics(ctx context.Context, m *Metrics) {
	if len(m.metrics) == 0 {
		return
	}
	payload := model.MetricsPayload{
		Service: s.tracer.Service,
		Process: s.tracer.process,
		System:  s.tracer.system,
		Metrics: m.metrics,
	}
	// TODO(axw) record tracer stats for metrics-sending?
	if err := s.tracer.Transport.SendMetrics(ctx, &payload); err != nil {
		if s.logger != nil {
			s.logger.Debugf("sending metrics failed: %s", err)
		}
	}
}

type metricsGatherer struct {
	MetricsGatherer
}

func (g *metricsGatherer) gather(ctx context.Context, m *Metrics) error {
	result := make(chan error, 1)
	go func() {
		defer close(result)
		defer func() {
			if r := recover(); r != nil {
				var err error
				if r, ok := r.(error); ok {
					err = r
				} else {
					err = errors.Errorf("%s", r)
				}
				result <- errors.Wrapf(err, "%T.GatherMetrics panicked", g)
			}
		}()
		result <- g.GatherMetrics(ctx, m)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err, ok := <-result:
		if !ok {
			return errors.New("metrics channel closed unexpectedly")
		}
		return err
	}
}
