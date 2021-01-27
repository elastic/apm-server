// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package sampling

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

const (
	badgerValueLogFileSize = 128 * 1024 * 1024

	// tooManyGroupsLoggerRateLimit is the maximum frequency at which
	// "too many groups" log messages are logged.
	tooManyGroupsLoggerRateLimit = time.Minute
)

// ErrStopped is returned when calling ProcessTransformables on a stopped Processor.
var ErrStopped = errors.New("processor is stopped")

// Processor is a tail-sampling event processor.
type Processor struct {
	config              Config
	logger              *logp.Logger
	tooManyGroupsLogger *logp.Logger
	groups              *traceGroups

	storageMu    sync.RWMutex
	db           *badger.DB
	storage      *eventstorage.ShardedReadWriter
	eventMetrics eventMetrics

	stopMu   sync.Mutex
	stopping chan struct{}
	stopped  chan struct{}
}

type eventMetrics struct {
	processed int64
	dropped   int64
	stored    int64
}

// NewProcessor returns a new Processor, for tail-sampling trace events.
func NewProcessor(config Config) (*Processor, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid tail-sampling config")
	}

	logger := logp.NewLogger(logs.Sampling)
	badgerOpts := badger.DefaultOptions(config.StorageDir)
	badgerOpts.ValueLogFileSize = config.ValueLogFileSize
	if badgerOpts.ValueLogFileSize == 0 {
		badgerOpts.ValueLogFileSize = badgerValueLogFileSize
	}
	badgerOpts.Logger = eventstorage.LogpAdaptor{Logger: logger}
	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, err
	}

	eventCodec := eventstorage.JSONCodec{}
	storage := eventstorage.New(db, eventCodec, config.TTL)
	readWriter := storage.NewShardedReadWriter()

	p := &Processor{
		config:              config,
		logger:              logger,
		tooManyGroupsLogger: logger.WithOptions(logs.WithRateLimit(tooManyGroupsLoggerRateLimit)),
		groups:              newTraceGroups(config.Policies, config.MaxDynamicServices, config.IngestRateDecayFactor),
		db:                  db,
		storage:             readWriter,
		stopping:            make(chan struct{}),
		stopped:             make(chan struct{}),
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
	numDynamicGroups := len(p.groups.dynamicGroups)
	p.groups.mu.RUnlock()
	monitoring.ReportInt(V, "dynamic_service_groups", int64(numDynamicGroups))

	monitoring.ReportNamespace(V, "storage", func() {
		p.storageMu.RLock()
		defer p.storageMu.RUnlock()
		lsmSize, valueLogSize := p.db.Size()
		monitoring.ReportInt(V, "lsm_size", int64(lsmSize))
		monitoring.ReportInt(V, "value_log_size", int64(valueLogSize))
	})
	monitoring.ReportNamespace(V, "events", func() {
		monitoring.ReportInt(V, "processed", atomic.LoadInt64(&p.eventMetrics.processed))
		monitoring.ReportInt(V, "dropped", atomic.LoadInt64(&p.eventMetrics.dropped))
		monitoring.ReportInt(V, "stored", atomic.LoadInt64(&p.eventMetrics.stored))
	})
}

// ProcessTransformables tail-samples transactions and spans.
//
// Any events returned by the processor will be published immediately.
// This includes:
//
// - Non-trace events (errors, metricsets)
// - Trace events which are already known to have been tail-sampled
// - Transactions which are head-based unsampled
//
// All other trace events will either be dropped (e.g. known to not
// be tail-sampled), or stored for possible later publication.
func (p *Processor) ProcessTransformables(ctx context.Context, events []transform.Transformable) ([]transform.Transformable, error) {
	p.storageMu.RLock()
	defer p.storageMu.RUnlock()
	if p.storage == nil {
		return nil, ErrStopped
	}
	for i := 0; i < len(events); i++ {
		var report, stored bool
		var err error
		switch event := events[i].(type) {
		case *model.Transaction:
			atomic.AddInt64(&p.eventMetrics.processed, 1)
			report, stored, err = p.processTransaction(event)
		case *model.Span:
			atomic.AddInt64(&p.eventMetrics.processed, 1)
			report, stored, err = p.processSpan(event)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		if !report {
			// We shouldn't report this event, so remove it from the slice.
			n := len(events)
			events[i], events[n-1] = events[n-1], events[i]
			events = events[:n-1]
			i--
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
	return events, nil
}

func (p *Processor) processTransaction(tx *model.Transaction) (report, stored bool, _ error) {
	if tx.Sampled != nil && !*tx.Sampled {
		// (Head-based) unsampled transactions are passed through
		// by the tail sampler.
		return true, false, nil
	}

	traceSampled, err := p.storage.IsTraceSampled(tx.TraceID)
	switch err {
	case nil:
		// Tail-sampling decision has been made: report the transaction
		// if it was sampled.
		report := traceSampled
		return report, false, nil
	case eventstorage.ErrNotFound:
		// Tail-sampling decision has not yet been made.
		break
	default:
		return false, false, err
	}

	if tx.ParentID != "" {
		// Non-root transaction: write to local storage while we wait
		// for a sampling decision.
		return false, true, p.storage.WriteTransaction(tx)
	}

	// Root transaction: apply reservoir sampling.
	reservoirSampled, err := p.groups.sampleTrace(tx)
	if err == errTooManyTraceGroups {
		// Too many trace groups, drop the transaction.
		p.tooManyGroupsLogger.Warn(`
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
		return false, false, p.storage.WriteTraceSampled(tx.TraceID, false)
	}

	// The root transaction was admitted to the sampling reservoir, so we
	// can proceed to write the transaction to storage; we may index it later,
	// after finalising the sampling decision.
	return false, true, p.storage.WriteTransaction(tx)
}

func (p *Processor) processSpan(span *model.Span) (report, stored bool, _ error) {
	traceSampled, err := p.storage.IsTraceSampled(span.TraceID)
	if err != nil {
		if err == eventstorage.ErrNotFound {
			// Tail-sampling decision has not yet been made, write span to local storage.
			return false, true, p.storage.WriteSpan(span)
		}
		return false, false, err
	}
	// Tail-sampling decision has been made, report or drop the event.
	if !traceSampled {
		return false, false, nil
	}
	return true, false, nil
}

// Stop stops the processor, flushing and closing the event storage.
func (p *Processor) Stop(ctx context.Context) error {
	p.stopMu.Lock()
	if p.storage == nil {
		// Already fully stopped.
		p.stopMu.Unlock()
		return nil
	}
	select {
	case <-p.stopping:
		// already stopping
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

	// Lock storage before stopping, to prevent closing
	// storage while ProcessTransformables is using it.
	p.storageMu.Lock()
	defer p.storageMu.Unlock()

	if err := p.storage.Flush(); err != nil {
		return err
	}
	p.storage.Close()
	if err := p.db.Close(); err != nil {
		return err
	}
	p.storage = nil
	return nil
}

// Run runs the tail-sampling processor. This method is responsible for:
//
//  - periodically making, and then publishing, local sampling decisions
//  - subscribing to remote sampling decisions
//  - reacting to both local and remote sampling decisions by reading
//    related events from local storage, and then reporting them
//
// Run returns when a fatal error occurs or the Stop method is invoked.
func (p *Processor) Run() error {
	p.storageMu.RLock()
	defer p.storageMu.RUnlock()
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

	pubsub, err := pubsub.New(pubsub.Config{
		BeatID: p.config.BeatID,
		Client: p.config.Elasticsearch,
		Index:  p.config.SampledTracesIndex,
		Logger: p.logger,

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
	errgroup, ctx := errgroup.WithContext(context.Background())
	errgroup.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.stopping:
			return context.Canceled
		}
	})
	errgroup.Go(func() error {
		// This goroutine is responsible for periodically garbage
		// collecting the Badger value log, using the recommended
		// discard ratio of 0.5.
		ticker := time.NewTicker(p.config.StorageGCInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				const discardRatio = 0.5
				if err := p.db.RunValueLogGC(discardRatio); err != nil && err != badger.ErrNoRewrite {
					return err
				}
			}
		}
	})
	errgroup.Go(func() error {
		return pubsub.SubscribeSampledTraceIDs(ctx, remoteSampledTraceIDs)
	})
	errgroup.Go(func() error {
		ticker := time.NewTicker(p.config.FlushInterval)
		defer ticker.Stop()
		var traceIDs []string
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				p.logger.Debug("finalizing local sampling reservoirs")
				traceIDs = p.groups.finalizeSampledTraces(traceIDs)
				if len(traceIDs) == 0 {
					continue
				}
				if err := pubsub.PublishSampledTraceIDs(ctx, traceIDs...); err != nil {
					return err
				}
				for _, traceID := range traceIDs {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case localSampledTraceIDs <- traceID:
					}
				}
				traceIDs = traceIDs[:0]
			}
		}
	})
	errgroup.Go(func() error {
		// TODO(axw) pace the publishing over the flush interval?
		// Alternatively we can rely on backpressure from the reporter,
		// removing the artificial one second timeout from publisher code
		// and just waiting as long as it takes here.
		var events model.Batch
		for {
			var remoteDecision bool
			var traceID string
			select {
			case <-ctx.Done():
				return ctx.Err()
			case traceID = <-remoteSampledTraceIDs:
				p.logger.Debug("received remotely sampled trace ID")
				remoteDecision = true
			case traceID = <-localSampledTraceIDs:
			}
			if err := p.storage.WriteTraceSampled(traceID, true); err != nil {
				return err
			}
			if err := p.storage.ReadEvents(traceID, &events); err != nil {
				return err
			}
			transformables := events.Transformables()
			if len(transformables) > 0 {
				p.logger.Debugf("reporting %d events", len(transformables))
				if remoteDecision {
					// Remote decisions may be received multiple times,
					// e.g. if this server restarts and resubscribes to
					// remote sampling decisions before they have been
					// deleted. We delete events from local storage so
					// we don't publish duplicates; delivery is therefore
					// at-most-once, not guaranteed.
					for _, tx := range events.Transactions {
						if err := p.storage.DeleteTransaction(tx); err != nil {
							return errors.Wrap(err, "failed to delete transaction from local storage")
						}
					}
					for _, span := range events.Spans {
						if err := p.storage.DeleteSpan(span); err != nil {
							return errors.Wrap(err, "failed to delete span from local storage")
						}
					}
				}
				if err := p.config.Reporter(ctx, publish.PendingReq{
					Transformables: transformables,
					Trace:          true,
				}); err != nil {
					p.logger.With(logp.Error(err)).Warn("failed to report events")
				}
			}
			events.Reset()
		}
	})
	if err := errgroup.Wait(); err != nil && err != context.Canceled {
		return err
	}
	return nil
}
