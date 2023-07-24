// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package sampling

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

const (
	// subscriberPositionFile holds the file name used for persisting
	// the subscriber position across server restarts.
	subscriberPositionFile = "subscriber_position.json"

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

	eventStore   *wrappedRW
	eventMetrics *eventMetrics // heap-allocated for 64-bit alignment

	stopMu   sync.Mutex
	stopping chan struct{}
	stopped  chan struct{}

	indexOnWriteFailure bool
}

type eventMetrics struct {
	processed     int64
	dropped       int64
	stored        int64
	sampled       int64
	headUnsampled int64
	failedWrites  int64
}

// NewProcessor returns a new Processor, for tail-sampling trace events.
func NewProcessor(config Config) (*Processor, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid tail-sampling config")
	}

	logger := logp.NewLogger(logs.Sampling)
	p := &Processor{
		config:            config,
		logger:            logger,
		rateLimitedLogger: logger.WithOptions(logs.WithRateLimit(loggerRateLimit)),
		groups:            newTraceGroups(config.Policies, config.MaxDynamicServices, config.IngestRateDecayFactor),
		eventStore:        newWrappedRW(config.Storage, config.TTL, int64(config.StorageLimit)),
		eventMetrics:      &eventMetrics{},
		stopping:          make(chan struct{}),
		stopped:           make(chan struct{}),
		// NOTE(marclop) This behavior should be configurable so users who
		// rely on tail sampling for cost cutting, can discard events once
		// the disk is full.
		// Index all traces when the storage limit is reached.
		indexOnWriteFailure: true,
	}
	return p, nil
}

// CollectMonitoring may be called to collect monitoring metrics related to
// tail-sampling. It is intended to be used with libbeat/monitoring.NewFunc.
//
// The metrics should be added to the "apm-server.sampling.tail" registry.
func (p *Processor) CollectMonitoring(_ monitoring.Mode, V monitoring.Visitor) {
	V.OnRegistryStart()
	defer V.OnRegistryFinished()

	// TODO(axw) it might be nice to also report some metrics about:
	//
	//   - The time between receiving events and when they are indexed.
	//     This could be accomplished by recording the time when the
	//     payload was received in the ECS field `event.created`. The
	//     final metric would ideally be a distribution, which is not
	//     currently an option in libbeat/monitoring.

	p.groups.mu.RLock()
	numDynamicGroups := p.groups.numDynamicServiceGroups
	p.groups.mu.RUnlock()
	monitoring.ReportInt(V, "dynamic_service_groups", int64(numDynamicGroups))

	monitoring.ReportNamespace(V, "storage", func() {
		lsmSize, valueLogSize := p.config.DB.Size()
		monitoring.ReportInt(V, "lsm_size", int64(lsmSize))
		monitoring.ReportInt(V, "value_log_size", int64(valueLogSize))
	})
	monitoring.ReportNamespace(V, "events", func() {
		monitoring.ReportInt(V, "processed", atomic.LoadInt64(&p.eventMetrics.processed))
		monitoring.ReportInt(V, "dropped", atomic.LoadInt64(&p.eventMetrics.dropped))
		monitoring.ReportInt(V, "stored", atomic.LoadInt64(&p.eventMetrics.stored))
		monitoring.ReportInt(V, "sampled", atomic.LoadInt64(&p.eventMetrics.sampled))
		monitoring.ReportInt(V, "head_unsampled", atomic.LoadInt64(&p.eventMetrics.headUnsampled))
		monitoring.ReportInt(V, "failed_writes", atomic.LoadInt64(&p.eventMetrics.failedWrites))
	})
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
			atomic.AddInt64(&p.eventMetrics.processed, 1)
			report, stored, err = p.processTransaction(event)
		case modelpb.SpanEventType:
			atomic.AddInt64(&p.eventMetrics.processed, 1)
			report, stored, err = p.processSpan(event)
		default:
			continue
		}

		// If processing the transaction or span returns with an error we
		// either discard or sample the trace by default.
		if err != nil {
			failed = true
			stored = false
			if p.indexOnWriteFailure {
				report = true
				p.rateLimitedLogger.Info("processing trace failed, indexing by default")
			} else {
				report = false
				p.rateLimitedLogger.Info("processing trace failed, discarding by default")
			}
		}

		if !report {
			// We shouldn't report this event, so remove it from the slice.
			n := len(events)
			events[i], events[n-1] = events[n-1], events[i]
			events = events[:n-1]
			i--
		}

		p.updateProcessorMetrics(report, stored, failed)
	}
	*batch = events
	return nil
}

func (p *Processor) updateProcessorMetrics(report, stored, failedWrite bool) {
	if failedWrite {
		atomic.AddInt64(&p.eventMetrics.failedWrites, 1)
	}
	if stored {
		atomic.AddInt64(&p.eventMetrics.stored, 1)
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
		atomic.AddInt64(&p.eventMetrics.dropped, 1)
	}
}

func (p *Processor) processTransaction(event *modelpb.APMEvent) (report, stored bool, _ error) {
	if !event.Transaction.Sampled {
		// (Head-based) unsampled transactions are passed through
		// by the tail sampler.
		atomic.AddInt64(&p.eventMetrics.headUnsampled, 1)
		return true, false, nil
	}

	traceSampled, err := p.eventStore.IsTraceSampled(event.Trace.Id)
	switch err {
	case nil:
		// Tail-sampling decision has been made: report the transaction
		// if it was sampled.
		report := traceSampled
		if report {
			atomic.AddInt64(&p.eventMetrics.sampled, 1)
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
		atomic.AddInt64(&p.eventMetrics.sampled, 1)
	}
	return traceSampled, false, nil
}

// Stop stops the processor, flushing event storage. Note that the underlying
// badger.DB must be closed independently to ensure writes are synced to disk.
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

	// Flush event store and the underlying read writers
	return p.eventStore.Flush()
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

	initialSubscriberPosition, err := readSubscriberPosition(p.logger, p.config.StorageDir)
	if err != nil {
		return err
	}
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
				if err := writeSubscriberPosition(p.config.StorageDir, pos); err != nil {
					p.rateLimitedLogger.With(logp.Error(err)).With(logp.Reflect("position", pos)).Warn(
						"failed to write subscriber position: %s", err,
					)
				}
			}
		}
	})
	g.Go(func() error {
		// This goroutine is responsible for periodically garbage
		// collecting the Badger value log, using the recommended
		// discard ratio of 0.5.
		ticker := time.NewTicker(p.config.StorageGCInterval)
		defer ticker.Stop()
		for {
			select {
			case <-p.stopping:
				return nil
			case <-ticker.C:
				const discardRatio = 0.5
				var err error
				for err == nil {
					// Keep garbage collecting until there are no more rewrites,
					// or garbage collection fails.
					err = p.config.DB.RunValueLogGC(discardRatio)
				}
				if err != nil && err != badger.ErrNoRewrite {
					return err
				}
			}
		}
	})
	g.Go(func() error {
		// Subscribe to remotely sampled trace IDs. This is cancelled immediately when
		// Stop is called. The next subscriber will pick up from the previous position.
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

			if err := p.eventStore.WriteTraceSampled(traceID, true); err != nil {
				p.rateLimitedLogger.Warnf(
					"received error writing sampled trace: %s", err,
				)
			}
			var events modelpb.Batch
			if err := p.eventStore.ReadTraceEvents(traceID, &events); err != nil {
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
					for _, event := range events {
						switch event.Type() {
						case modelpb.TransactionEventType:
							if err := p.eventStore.DeleteTraceEvent(event.Trace.Id, event.Transaction.Id); err != nil {
								return errors.Wrap(err, "failed to delete transaction from local storage")
							}
						case modelpb.SpanEventType:
							if err := p.eventStore.DeleteTraceEvent(event.Trace.Id, event.Span.Id); err != nil {
								return errors.Wrap(err, "failed to delete span from local storage")
							}
						}
					}
				}
				atomic.AddInt64(&p.eventMetrics.sampled, int64(len(events)))
				if err := p.config.BatchProcessor.ProcessBatch(gracefulContext, &events); err != nil {
					p.logger.With(logp.Error(err)).Warn("failed to report events")
				}
			}
		}
	})
	if err := g.Wait(); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func readSubscriberPosition(logger *logp.Logger, storageDir string) (pubsub.SubscriberPosition, error) {
	var pos pubsub.SubscriberPosition
	data, err := os.ReadFile(filepath.Join(storageDir, subscriberPositionFile))
	if errors.Is(err, os.ErrNotExist) {
		return pos, nil
	} else if err != nil {
		return pos, err
	}
	err = json.Unmarshal(data, &pos)
	if err != nil {
		logger.With(logp.Error(err)).With(logp.ByteString("file", data)).Debug("failed to read subscriber position")
	}
	return pos, err
}

func writeSubscriberPosition(storageDir string, pos pubsub.SubscriberPosition) error {
	data, err := json.Marshal(pos)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(storageDir, subscriberPositionFile), data, 0644)
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

const (
	storageLimitThreshold = 0.90 // Allow 90% of the quota to be used.
)

// wrappedRW wraps configurable write options for global ShardedReadWriter
type wrappedRW struct {
	rw         *eventstorage.ShardedReadWriter
	writerOpts eventstorage.WriterOpts
}

// Stored entries expire after ttl.
// The amount of storage that can be consumed can be limited by passing in a
// limit value greater than zero. The hard limit on storage is set to 90% of
// the limit to account for delay in the size reporting by badger.
// https://github.com/dgraph-io/badger/blob/82b00f27e3827022082225221ae05c03f0d37620/db.go#L1302-L1319.
func newWrappedRW(rw *eventstorage.ShardedReadWriter, ttl time.Duration, limit int64) *wrappedRW {
	if limit > 1 {
		limit = int64(float64(limit) * storageLimitThreshold)
	}
	return &wrappedRW{
		rw: rw,
		writerOpts: eventstorage.WriterOpts{
			TTL:                 ttl,
			StorageLimitInBytes: limit,
		},
	}
}

// ReadTraceEvents calls ShardedReadWriter.ReadTraceEvents
func (s *wrappedRW) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	return s.rw.ReadTraceEvents(traceID, out)
}

// WriteTraceEvents calls ShardedReadWriter.WriteTraceEvents using the configured WriterOpts
func (s *wrappedRW) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent) error {
	return s.rw.WriteTraceEvent(traceID, id, event, s.writerOpts)
}

// WriteTraceSampled calls ShardedReadWriter.WriteTraceSampled using the configured WriterOpts
func (s *wrappedRW) WriteTraceSampled(traceID string, sampled bool) error {
	return s.rw.WriteTraceSampled(traceID, sampled, s.writerOpts)
}

// IsTraceSampled calls ShardedReadWriter.IsTraceSampled
func (s *wrappedRW) IsTraceSampled(traceID string) (bool, error) {
	return s.rw.IsTraceSampled(traceID)
}

// DeleteTraceEvent calls ShardedReadWriter.DeleteTraceEvent
func (s *wrappedRW) DeleteTraceEvent(traceID, id string) error {
	return s.rw.DeleteTraceEvent(traceID, id)
}

// Flush calls ShardedReadWriter.Flush
func (s *wrappedRW) Flush() error {
	return s.rw.Flush(s.writerOpts.StorageLimitInBytes)
}
