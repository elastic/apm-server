// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package sampling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/elastic/beats/v7/x-pack/libbeat/statusreporterhelper"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-data/model/modelpb"

	"github.com/elastic/beats/v7/libbeat/management/status"
	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub"
)

const (
	// loggerRateLimit is the maximum frequency at which "too many groups" and
	// "write failure" log messages are logged.
	loggerRateLimit = time.Minute

	// shutdownGracePeriod is the time that the processor has to gracefully
	// terminate after the stop method is called.
	shutdownGracePeriod = 5 * time.Second
)

// Processor is a tail-sampling event processor.
type Processor struct {
	config            Config
	logger            *logp.Logger
	rateLimitedLogger *logp.Logger
	groups            *traceGroups

	eventStore     eventstorage.RW
	eventMetrics   eventMetrics
	shardLock      *shardLock
	statusReporter status.StatusReporter

	stopMu   sync.Mutex
	stopping chan struct{}
	stopped  chan struct{}
}

type ProcessorParams struct {
	Config         Config
	Logger         *logp.Logger
	StatusReporter status.StatusReporter
}

type eventMetrics struct {
	processed     metric.Int64Counter
	dropped       metric.Int64Counter
	stored        metric.Int64Counter
	sampled       metric.Int64Counter
	headUnsampled metric.Int64Counter
	failedWrites  metric.Int64Counter
}

// NewProcessor returns a new Processor, for tail-sampling trace events.
func NewProcessor(params ProcessorParams) (*Processor, error) {
	config := params.Config
	logger := params.Logger
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tail-sampling config: %w", err)
	}

	meter := config.MeterProvider.Meter("github.com/elastic/apm-server/x-pack/apm-server/sampling")

	logger = logger.Named(logs.Sampling)
	p := &Processor{
		config:            config,
		logger:            logger,
		rateLimitedLogger: logger.WithOptions(logs.WithRateLimit(loggerRateLimit)),
		groups:            newTraceGroups(meter, config.Policies, config.MaxDynamicServices, config.IngestRateDecayFactor),
		eventStore:        config.Storage,
		shardLock:         newShardLock(runtime.GOMAXPROCS(0)),
		statusReporter:    statusreporterhelper.New(params.StatusReporter, params.Logger, "apm sampling processor"),
		stopping:          make(chan struct{}),
		stopped:           make(chan struct{}),
	}

	p.eventMetrics.processed, _ = meter.Int64Counter("apm-server.sampling.tail.events.processed")
	p.eventMetrics.dropped, _ = meter.Int64Counter("apm-server.sampling.tail.events.dropped")
	p.eventMetrics.stored, _ = meter.Int64Counter("apm-server.sampling.tail.events.stored")
	p.eventMetrics.sampled, _ = meter.Int64Counter("apm-server.sampling.tail.events.sampled")
	p.eventMetrics.headUnsampled, _ = meter.Int64Counter("apm-server.sampling.tail.events.head_unsampled")
	p.eventMetrics.failedWrites, _ = meter.Int64Counter("apm-server.sampling.tail.events.failed_writes")

	return p, nil
}

// ProcessBatch tail-samples transactions and spans.
//
// Any events remaining in the batch after the processor returns
// will be published immediately. This includes:
//
// - Non-trace events (errors, metricsets)
// - Trace events which are already known to have been tail-sampled
// - Transactions which are head-based unsampled
//
// All other trace events will either be dropped (e.g. known to not
// be tail-sampled), or stored for possible later publication.
func (p *Processor) ProcessBatch(ctx context.Context, batch *modelpb.Batch) error {
	events := *batch
	for i := 0; i < len(events); i++ {
		event := events[i]
		var report, stored, failed bool
		var err error
		switch event.Type() {
		case modelpb.TransactionEventType:
			p.eventMetrics.processed.Add(context.Background(), 1)
			report, stored, err = p.processTransaction(event)
		case modelpb.SpanEventType:
			p.eventMetrics.processed.Add(context.Background(), 1)
			report, stored, err = p.processSpan(event)
		default:
			continue
		}

		// If processing the transaction or span returns with an error we
		// either discard or sample the trace by default.
		if err != nil {
			failed = true
			stored = false
			var msg string
			if p.config.DiscardOnWriteFailure {
				report = false
				msg = "processing trace failed, discarding by default"
			} else {
				report = true
				msg = "processing trace failed, indexing by default"
			}
			p.rateLimitedLogger.With(logp.Error(err)).Warn(msg)
			p.statusReporter.UpdateStatus(status.Degraded, "sampling: "+msg+": "+err.Error())
		}

		if !report {
			// We shouldn't report this event, so remove it from the slice.
			n := len(events)
			events[i], events[n-1] = events[n-1], events[i]
			events = events[:n-1]
			i--
		}

		if stored {
			p.statusReporter.UpdateStatus(status.Running, "")
		}

		p.updateProcessorMetrics(report, stored, failed)
	}
	*batch = events
	return nil
}

func (p *Processor) updateProcessorMetrics(report, stored, failedWrite bool) {
	if failedWrite {
		p.eventMetrics.failedWrites.Add(context.Background(), 1)
	}
	if stored {
		p.eventMetrics.stored.Add(context.Background(), 1)
	} else if !report {
		// We only increment the "dropped" counter if
		// we neither reported nor stored the event, so
		// we can track how many events are definitely
		// dropped and indexing isn't just deferred until
		// later.
		//
		// The counter does not include events that are
		// implicitly dropped, i.e. stored and never
		// indexed.
		p.eventMetrics.dropped.Add(context.Background(), 1)
	}
}

func (p *Processor) processTransaction(event *modelpb.APMEvent) (report, stored bool, _ error) {
	if !event.Transaction.Sampled {
		// (Head-based) unsampled transactions are passed through
		// by the tail sampler.
		p.eventMetrics.headUnsampled.Add(context.Background(), 1)
		return true, false, nil
	}

	p.shardLock.RLock(event.Trace.Id)
	defer p.shardLock.RUnlock(event.Trace.Id)

	traceSampled, err := p.eventStore.IsTraceSampled(event.Trace.Id)
	switch err {
	case nil:
		// Tail-sampling decision has been made: report the transaction
		// if it was sampled.
		report := traceSampled
		if report {
			p.eventMetrics.sampled.Add(context.Background(), 1)
		}
		return report, false, nil
	case eventstorage.ErrNotFound:
		// Tail-sampling decision has not yet been made.
		break
	default:
		return false, false, err
	}

	if event.GetParentId() != "" {
		// Non-root transaction: write to local storage while we wait
		// for a sampling decision.
		return false, true, p.eventStore.WriteTraceEvent(
			event.Trace.Id, event.Transaction.Id, event,
		)
	}

	// Root transaction: apply reservoir sampling.
	//
	// TODO(axw) we should skip reservoir sampling when the matching
	// policy's sampling rate is 100%, immediately index the event
	// and record the trace sampling decision.
	reservoirSampled, err := p.groups.sampleTrace(event)
	if err == errTooManyTraceGroups {
		// Too many trace groups, drop the transaction.
		p.rateLimitedLogger.Warn(`
Tail-sampling service group limit reached, discarding trace events.
This is caused by having many unique service names while relying on
sampling policies without service name specified.
`[1:])
		return false, false, nil
	} else if err != nil {
		return false, false, err
	}

	if !reservoirSampled {
		// Write the non-sampling decision to storage to avoid further
		// writes for the trace ID, and then drop the transaction.
		//
		// This is a local optimisation only. To avoid creating network
		// traffic and load on Elasticsearch for uninteresting root
		// transactions, we do not propagate this to other APM Servers.
		return false, false, p.eventStore.WriteTraceSampled(event.Trace.Id, false)
	}

	// The root transaction was admitted to the sampling reservoir, so we
	// can proceed to write the transaction to storage; we may index it later,
	// after finalising the sampling decision.
	return false, true, p.eventStore.WriteTraceEvent(event.Trace.Id, event.Transaction.Id, event)
}

func (p *Processor) processSpan(event *modelpb.APMEvent) (report, stored bool, _ error) {
	p.shardLock.RLock(event.Trace.Id)
	defer p.shardLock.RUnlock(event.Trace.Id)

	traceSampled, err := p.eventStore.IsTraceSampled(event.Trace.Id)
	if err != nil {
		if err == eventstorage.ErrNotFound {
			// Tail-sampling decision has not yet been made, write event to local storage.
			return false, true, p.eventStore.WriteTraceEvent(event.Trace.Id, event.Span.Id, event)
		}
		return false, false, err
	}
	// Tail-sampling decision has been made, report or drop the event.
	if traceSampled {
		p.eventMetrics.sampled.Add(context.Background(), 1)
	}
	return traceSampled, false, nil
}

// Stop stops the processor.
// Note that the underlying StorageManager must be closed independently
// to ensure writes are synced to disk.
func (p *Processor) Stop(ctx context.Context) error {
	p.stopMu.Lock()
	select {
	case <-p.stopped:
	case <-p.stopping:
		// already stopped or stopping
	default:
		close(p.stopping)
	}
	p.stopMu.Unlock()

	// Wait for Run to return.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.stopped:
	}

	return nil
}

// Run runs the tail-sampling processor. This method is responsible for:
//
//   - periodically making, and then publishing, local sampling decisions
//   - subscribing to remote sampling decisions
//   - reacting to both local and remote sampling decisions by reading
//     related events from local storage, and then reporting them
//
// Run returns when a fatal error occurs or the Stop method is invoked.
func (p *Processor) Run() error {
	defer func() {
		p.stopMu.Lock()
		defer p.stopMu.Unlock()
		select {
		case <-p.stopped:
		default:
			close(p.stopped)
		}
	}()

	// NOTE(axw) the user can configure the tail-sampling flush interval,
	// but cannot directly control the bulk indexing flush interval. The
	// bulk indexing is expected to complete soon after the tail-sampling
	// flush interval.
	bulkIndexerFlushInterval := 5 * time.Second
	if bulkIndexerFlushInterval > p.config.FlushInterval {
		bulkIndexerFlushInterval = p.config.FlushInterval
	}

	initialSubscriberPosition := readSubscriberPosition(p.logger, p.config.DB)
	subscriberPositions := make(chan pubsub.SubscriberPosition)
	pubsub, err := pubsub.New(pubsub.Config{
		ServerID:   p.config.UUID,
		Client:     p.config.Elasticsearch,
		DataStream: pubsub.DataStreamConfig(p.config.SampledTracesDataStream),
		Logger:     p.logger,

		// Issue pubsub subscriber search requests at twice the frequency
		// of publishing, so each server observes each other's sampled
		// trace IDs soon after they are published.
		SearchInterval: p.config.FlushInterval / 2,
		FlushInterval:  bulkIndexerFlushInterval,
	})
	if err != nil {
		return err
	}

	remoteSampledTraceIDs := make(chan string)
	localSampledTraceIDs := make(chan string)
	publishSampledTraceIDs := make(chan string)
	gracefulContext, cancelGracefulContext := context.WithCancel(context.Background())
	defer cancelGracefulContext()
	var g errgroup.Group
	g.Go(func() error {
		// Write subscriber position to a file on disk, to support resuming
		// on apm-server restart without reprocessing all indices. We trigger
		// the graceful shutdown from this goroutine to ensure we do not
		// write any subscriber positions after Stop is called, and risk
		// having a new subscriber miss events.
		for {
			select {
			case <-p.stopping:
				time.AfterFunc(shutdownGracePeriod, cancelGracefulContext)
				return context.Canceled
			case pos := <-subscriberPositions:
				if err := writeSubscriberPosition(p.config.DB, pos); err != nil {
					p.rateLimitedLogger.With(logp.Error(err)).With(logp.Reflect("position", pos)).Warnf(
						"failed to write subscriber position: %s", err,
					)
				}
			}
		}
	})
	g.Go(func() error {
		return p.config.DB.Run(p.stopping, p.config.TTL)
	})
	g.Go(func() error {
		// Subscribe to remotely sampled trace IDs. This is cancelled immediately when
		// Stop is called. But it is possible that both old and new subscriber goroutines
		// run concurrently, before the old one eventually receives the Stop call.
		// The next subscriber will pick up from the previous position.
		defer close(remoteSampledTraceIDs)
		defer close(subscriberPositions)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			defer cancel()
			select {
			case <-p.stopping:
			case <-p.stopped:
			}
		}()
		return pubsub.SubscribeSampledTraceIDs(
			ctx, initialSubscriberPosition, remoteSampledTraceIDs, subscriberPositions,
		)
	})
	g.Go(func() error {
		// Publish locally sampled trace IDs to Elasticsearch. This is cancelled when
		// publishSampledTraceIDs is closed, after the final reservoir flush.
		return pubsub.PublishSampledTraceIDs(gracefulContext, publishSampledTraceIDs)
	})
	g.Go(func() error {
		ticker := time.NewTicker(p.config.FlushInterval)
		defer ticker.Stop()
		var traceIDs []string

		// Close publishSampledTraceIDs and localSampledTraceIDs after returning,
		// which implies that either all decisions have been published or the grace
		// period has elapsed. This will unblock the PublishSampledTraceIDs call above,
		// and the event indexing goroutine below.
		defer close(publishSampledTraceIDs)
		defer close(localSampledTraceIDs)

		publishDecisions := func() error {
			p.logger.Debug("finalizing local sampling reservoirs")
			traceIDs = p.groups.finalizeSampledTraces(traceIDs)
			if len(traceIDs) == 0 {
				return nil
			}
			var g errgroup.Group
			g.Go(func() error { return sendTraceIDs(gracefulContext, publishSampledTraceIDs, traceIDs) })
			g.Go(func() error { return sendTraceIDs(gracefulContext, localSampledTraceIDs, traceIDs) })
			if err := g.Wait(); err != nil {
				return err
			}
			traceIDs = traceIDs[:0]
			return nil
		}

		for {
			select {
			case <-p.stopping:
				return publishDecisions()
			case <-ticker.C:
				if err := publishDecisions(); err != nil {
					return err
				}
			}
		}
	})
	g.Go(func() error {
		var events modelpb.Batch
		// TODO(axw) pace the publishing over the flush interval?
		// Alternatively we can rely on backpressure from the reporter,
		// removing the artificial one second timeout from publisher code
		// and just waiting as long as it takes here.
		remoteSampledTraceIDs := remoteSampledTraceIDs
		localSampledTraceIDs := localSampledTraceIDs
		for {
			if remoteSampledTraceIDs == nil && localSampledTraceIDs == nil {
				// The pubsub subscriber and reservoir finalizer have
				// both stopped, so there's nothing else to do.
				return nil
			}
			var remoteDecision bool
			var traceID string
			var ok bool
			select {
			case <-gracefulContext.Done():
				return gracefulContext.Err()
			case traceID, ok = <-remoteSampledTraceIDs:
				if !ok {
					remoteSampledTraceIDs = nil
					continue
				}
				p.logger.Debug("received remotely sampled trace ID")
				remoteDecision = true
			case traceID, ok = <-localSampledTraceIDs:
				if !ok {
					localSampledTraceIDs = nil
					continue
				}
			}

			// We lock before WriteTraceSampled here to prevent race condition with IsTraceSampled from incoming events.
			p.shardLock.Lock(traceID)
			if err := p.eventStore.WriteTraceSampled(traceID, true); err != nil {
				p.rateLimitedLogger.Warnf(
					"received error writing sampled trace: %s", err,
				)
			}
			p.shardLock.Unlock(traceID)

			events = events[:0]
			err = p.eventStore.ReadTraceEvents(traceID, &events)
			if err != nil {
				p.rateLimitedLogger.Warnf(
					"received error reading trace events: %s", err,
				)
				continue
			}
			if n := len(events); n > 0 {
				p.logger.Debugf("reporting %d events", n)
				if remoteDecision {
					// Remote decisions may be received multiple times,
					// e.g. if this server restarts and resubscribes to
					// remote sampling decisions before they have been
					// deleted. We delete events from local storage so
					// we don't publish duplicates; delivery is therefore
					// at-most-once, not guaranteed.
					//
					// TODO(carsonip): pebble supports range deletes and may be better than
					// deleting events separately, but as we do not use transactions, it is
					// possible to race and delete something that is not read.
					for _, event := range events {
						switch event.Type() {
						case modelpb.TransactionEventType:
							if err := p.eventStore.DeleteTraceEvent(event.Trace.Id, event.Transaction.Id); err != nil {
								p.logger.With(logp.Error(err)).Warn("failed to delete transaction from local storage")
							}
						case modelpb.SpanEventType:
							if err := p.eventStore.DeleteTraceEvent(event.Trace.Id, event.Span.Id); err != nil {
								p.logger.With(logp.Error(err)).Warn("failed to delete span from local storage")
							}
						}
					}
				}
				p.eventMetrics.sampled.Add(gracefulContext, int64(len(events)))
				if err := p.config.BatchProcessor.ProcessBatch(gracefulContext, &events); err != nil {
					p.logger.With(logp.Error(err)).Warn("failed to report events")
				}

				for i := range events {
					events[i] = nil // not required but ensure that there is no ref to the freed event
				}
			}
		}
	})
	if err := g.Wait(); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func readSubscriberPosition(logger *logp.Logger, s *eventstorage.StorageManager) pubsub.SubscriberPosition {
	var pos pubsub.SubscriberPosition
	data, err := s.ReadSubscriberPosition()
	if errors.Is(err, os.ErrNotExist) {
		return pos
	} else if err != nil {
		logger.With(logp.Error(err)).Warn("error reading subscriber position file; proceeding as empty")
		return pos
	}
	err = json.Unmarshal(data, &pos)
	if err != nil {
		logger.With(logp.Error(err)).With(logp.ByteString("file", data)).Debug("failed to read subscriber position")
		logger.With(logp.Error(err)).Warn("error parsing subscriber position file; proceeding as empty")
		return pos
	}
	return pos
}

func writeSubscriberPosition(s *eventstorage.StorageManager, pos pubsub.SubscriberPosition) error {
	data, err := json.Marshal(pos)
	if err != nil {
		return err
	}

	return s.WriteSubscriberPosition(data)
}

func sendTraceIDs(ctx context.Context, out chan<- string, traceIDs []string) error {
	for _, traceID := range traceIDs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- traceID:
		}
	}
	return nil
}

type shardLock struct {
	locks []sync.RWMutex
}

func newShardLock(numShards int) *shardLock {
	if numShards <= 0 {
		panic("shardLock numShards must be greater than zero")
	}
	locks := make([]sync.RWMutex, numShards)
	return &shardLock{locks: locks}
}

func (s *shardLock) Lock(id string) {
	s.getLock(id).Lock()
}

func (s *shardLock) Unlock(id string) {
	s.getLock(id).Unlock()
}

func (s *shardLock) RLock(id string) {
	s.getLock(id).RLock()
}

func (s *shardLock) RUnlock(id string) {
	s.getLock(id).RUnlock()
}

func (s *shardLock) getLock(id string) *sync.RWMutex {
	var h xxhash.Digest
	_, _ = h.WriteString(id)
	return &s.locks[h.Sum64()%uint64(len(s.locks))]
}
