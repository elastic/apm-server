// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package modelindexer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.elastic.co/apm/module/apmzap/v2"
	"go.elastic.co/apm/v2"
	"go.elastic.co/fastjson"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/mapstr"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/internal/model"
)

const (
	logRateLimit = time.Minute

	// timestampFormat formats timestamps according to Elasticsearch's
	// strict_date_optional_time date format, which includes a fractional
	// seconds component.
	timestampFormat = "2006-01-02T15:04:05.000Z07:00"
)

// ErrClosed is returned from methods of closed Indexers.
var ErrClosed = errors.New("model indexer closed")

// Indexer is a model.BatchProcessor which bulk indexes events as Elasticsearch documents.
//
// Indexer buffers events in their JSON encoding until either the accumulated buffer reaches
// `config.FlushBytes`, or `config.FlushInterval` elapses.
//
// Indexer fills a single bulk request buffer at a time to ensure bulk requests are optimally
// sized, avoiding sparse bulk requests as much as possible. After a bulk request is flushed,
// the next event added will wait for the next available bulk request buffer and repeat the
// process.
//
// Up to `config.MaxRequests` bulk requests may be flushing/active concurrently, to allow the
// server to make progress encoding while Elasticsearch is busy servicing flushed bulk requests.
type Indexer struct {
	bulkRequests          int64
	eventsAdded           int64
	eventsActive          int64
	eventsFailed          int64
	eventsIndexed         int64
	tooManyRequests       int64
	bytesTotal            int64
	availableBulkRequests int64
	activeCreated         int64
	activeDestroyed       int64

	scalingInfo atomic.Value

	config                Config
	logger                *logp.Logger
	available             chan *bulkIndexer
	bulkItems             chan elasticsearch.BulkIndexerItem
	errgroup              errgroup.Group
	errgroupContext       context.Context
	cancelErrgroupContext context.CancelFunc

	mu     sync.Mutex
	closed chan struct{}
}

// Config holds configuration for Indexer.
type Config struct {
	// Tracer holds an optional apm.Tracer to use for tracing bulk requests
	// to Elasticsearch. Each bulk request is traced as a transaction.
	//
	// If Tracer is nil, requests will not be traced.
	Tracer *apm.Tracer

	// CompressionLevel holds the gzip compression level, from 0 (gzip.NoCompression)
	// to 9 (gzip.BestCompression). Higher values provide greater compression, at a
	// greater cost of CPU. The special value -1 (gzip.DefaultCompression) selects the
	// default compression level.
	CompressionLevel int

	// MaxRequests holds the maximum number of bulk index requests to execute concurrently.
	// The maximum memory usage of Indexer is thus approximately MaxRequests*FlushBytes.
	//
	// If MaxRequests is less than or equal to zero, the default of 10 will be used.
	MaxRequests int

	// FlushBytes holds the flush threshold in bytes. If Compression is enabled,
	// The number of events that can be buffered will be greater.
	//
	// If FlushBytes is zero, the default of 1MB will be used.
	FlushBytes int

	// FlushInterval holds the flush threshold as a duration.
	//
	// If FlushInterval is zero, the default of 30 seconds will be used.
	FlushInterval time.Duration

	// EventBufferSize sets the number of events that can be buffered before
	// they are stored in the active indexer buffer.
	//
	// If EventBufferSize is zero, the default 100 will be used.
	EventBufferSize int

	// Tracer holds an optional apm.Tracer to use for tracing bulk requests
	// to Elasticsearch. Each bulk request is traced as a transaction.
	// Scaling configuration for the modelindexer.
	//
	// If unspecified, scaling is enabled by default.
	Scaling ScalingConfig
}

// ScalingConfig holds the modelindexer autoscaling configuration.
type ScalingConfig struct {
	// Disabled toggles active indexer scaling on.
	//
	// It is enabled by default.
	Disabled bool

	// ScaleDown configures the Threshold and CoolDown for the scale down
	// action. In order to scale down an active indexer, the Threshold has
	// to be met after the CoolDown has elapsed. Scale down will only take
	// place if there are more than 1 active indexer.
	// Active indexers will be destroyed when they aren't needed anymore,
	// when enough timed flushes (FlushInterval) are performed by an active
	// indexer (controlled by Threshold), or when an active indexer is idle
	// for (IdleInterval * Threshold) as long as CoolDown allows it.
	//
	// When unset, the default of 30 is used for Threshold, and 30 seconds for
	// CoolDown.
	ScaleDown ScaleActionConfig

	// ScaleUp configures the Threshold and CoolDown for the scale up action.
	// In order for a scale up to occur, the Threshold has to be met after
	// the CoolDown has elapsed. By default,
	// a single active indexer is created which actively pulls events from the
	// internal buffered queue. When enough full flushes (FlushBytes) are
	// performed by an active indexer (controlled by Threshold), a new active
	// indexer will be created until GOMAXPROCS / 4 is reached (25% of CPU
	// capacity) if the CoolDown allows it.
	//
	// When unspecified, the default of 60 is used for Threshold, and 60 seconds
	// for CoolDown.
	ScaleUp ScaleActionConfig

	// IdleInterval defines how long an active indexer performs an inactivity
	// check. The ScaleDown.Threshold and ScaleDown.CoolDown needs to be met
	// for an active indexer to be destroyed.
	//
	// When unspecified, the default of 30 seconds will be used.
	IdleInterval time.Duration
}

// ScaleActionConfig holds the configuration for a scaling action
type ScaleActionConfig struct {
	// Threshold is the number of consecutive times a scale up/down condition
	// has to happen for the scaling action will be triggered.
	Threshold uint

	// CoolDown is the amount of time needed to elapse between scaling actions
	// to trigger it.
	CoolDown time.Duration
}

// New returns a new Indexer that indexes events directly into data streams.
func New(client elasticsearch.Client, cfg Config) (*Indexer, error) {
	logger := logp.NewLogger("modelindexer", logs.WithRateLimit(logRateLimit))
	if cfg.CompressionLevel < -1 || cfg.CompressionLevel > 9 {
		return nil, fmt.Errorf(
			"expected CompressionLevel in range [-1,9], got %d",
			cfg.CompressionLevel,
		)
	}
	if cfg.MaxRequests <= 0 {
		cfg.MaxRequests = 10
	}
	if cfg.FlushBytes <= 0 {
		cfg.FlushBytes = 1 * 1024 * 1024
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 30 * time.Second
	}
	if cfg.EventBufferSize <= 0 {
		cfg.EventBufferSize = 1024
	}
	if !cfg.Scaling.Disabled {
		if cfg.Scaling.ScaleDown.Threshold == 0 {
			cfg.Scaling.ScaleDown.Threshold = 30
		}
		if cfg.Scaling.ScaleDown.CoolDown <= 0 {
			cfg.Scaling.ScaleDown.CoolDown = 30 * time.Second
		}
		if cfg.Scaling.ScaleUp.Threshold == 0 {
			cfg.Scaling.ScaleUp.Threshold = 60
		}
		if cfg.Scaling.ScaleUp.CoolDown <= 0 {
			cfg.Scaling.ScaleUp.CoolDown = time.Minute
		}
		if cfg.Scaling.IdleInterval <= 0 {
			cfg.Scaling.IdleInterval = 30 * time.Second
		}
	}
	available := make(chan *bulkIndexer, cfg.MaxRequests)
	for i := 0; i < cfg.MaxRequests; i++ {
		available <- newBulkIndexer(client, cfg.CompressionLevel)
	}
	indexer := &Indexer{
		availableBulkRequests: int64(len(available)),
		config:                cfg,
		logger:                logger,
		available:             available,
		closed:                make(chan struct{}),
		// NOTE(marclop) This channel size is arbitrary.
		bulkItems: make(chan elasticsearch.BulkIndexerItem, cfg.EventBufferSize),
	}

	// We create a cancellable context for the errgroup.Group for unblocking
	// flushes when Close returns. We intentionally do not use errgroup.WithContext,
	// because one flush failure should not cause the context to be cancelled.
	indexer.errgroupContext, indexer.cancelErrgroupContext = context.WithCancel(
		context.Background(),
	)
	indexer.scalingInfo.Store(scalingInfo{activeIndexers: 1})
	indexer.errgroup.Go(func() error {
		indexer.runActiveIndexer()
		return nil
	})
	return indexer, nil
}

// Close closes the indexer, first flushing any queued events.
//
// Close returns an error if any flush attempts during the indexer's
// lifetime returned an error. If ctx is cancelled, Close returns and
// any ongoing flush attempts are cancelled.
func (i *Indexer) Close(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	select {
	case <-i.closed:
	default:
		close(i.closed)

		// Cancel ongoing flushes when ctx is cancelled.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			defer i.cancelErrgroupContext()
			<-ctx.Done()
		}()
	}
	return i.errgroup.Wait()
}

// Stats returns the bulk indexing stats.
func (i *Indexer) Stats() Stats {
	return Stats{
		Added:                 atomic.LoadInt64(&i.eventsAdded),
		Active:                atomic.LoadInt64(&i.eventsActive),
		BulkRequests:          atomic.LoadInt64(&i.bulkRequests),
		Failed:                atomic.LoadInt64(&i.eventsFailed),
		Indexed:               atomic.LoadInt64(&i.eventsIndexed),
		TooManyRequests:       atomic.LoadInt64(&i.tooManyRequests),
		BytesTotal:            atomic.LoadInt64(&i.bytesTotal),
		AvailableBulkRequests: atomic.LoadInt64(&i.availableBulkRequests),
		IndexersActive:        i.scalingInformation().activeIndexers,
		IndexersCreated:       atomic.LoadInt64(&i.activeCreated),
		IndexersDestroyed:     atomic.LoadInt64(&i.activeDestroyed),
	}
}

// ProcessBatch creates a document for each event in batch, and adds them to the
// Elasticsearch bulk indexer.
//
// If Close is called, then ProcessBatch will return ErrClosed.
func (i *Indexer) ProcessBatch(ctx context.Context, batch *model.Batch) error {
	for _, event := range *batch {
		if err := i.processEvent(ctx, &event); err != nil {
			return err
		}
	}
	return nil
}

func (i *Indexer) processEvent(ctx context.Context, event *model.APMEvent) error {
	r := getPooledReader()
	beatEvent := event.BeatEvent()
	if err := encodeBeatEvent(beatEvent, &r.jsonw); err != nil {
		return err
	}
	r.reader.Reset(r.jsonw.Bytes())

	r.indexBuilder.WriteString(event.DataStream.Type)
	r.indexBuilder.WriteByte('-')
	r.indexBuilder.WriteString(event.DataStream.Dataset)
	r.indexBuilder.WriteByte('-')
	r.indexBuilder.WriteString(event.DataStream.Namespace)

	// Send the BulkIndexerItem to the internal channel, allowing individual
	// events to be processed by an active bulk indexer in a dedicated goroutine,
	// which in turn speeds up event processing.
	item := elasticsearch.BulkIndexerItem{
		Index:  r.indexBuilder.String(),
		Action: "create",
		Body:   r,
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-i.closed:
		return ErrClosed
	case i.bulkItems <- item:
	}
	atomic.AddInt64(&i.eventsAdded, 1)
	atomic.AddInt64(&i.eventsActive, 1)
	return nil
}

func encodeBeatEvent(in beat.Event, out *fastjson.Writer) error {
	out.RawByte('{')
	out.RawString(`"@timestamp":"`)
	out.Time(in.Timestamp, timestampFormat)
	out.RawByte('"')
	for k, v := range in.Fields {
		out.RawByte(',')
		out.String(k)
		out.RawByte(':')
		if err := encodeAny(v, out); err != nil {
			return err
		}
	}
	out.RawByte('}')
	return nil
}

func encodeAny(v interface{}, out *fastjson.Writer) error {
	switch v := v.(type) {
	case mapstr.M:
		return encodeMap(v, out)
	case map[string]interface{}:
		return encodeMap(v, out)
	default:
		return fastjson.Marshal(out, v)
	}
}

func encodeMap(v map[string]interface{}, out *fastjson.Writer) error {
	out.RawByte('{')
	first := true
	for k, v := range v {
		if first {
			first = false
		} else {
			out.RawByte(',')
		}
		out.String(k)
		out.RawByte(':')
		if err := encodeAny(v, out); err != nil {
			return err
		}
	}
	out.RawByte('}')
	return nil
}

func (i *Indexer) flush(ctx context.Context, bulkIndexer *bulkIndexer) error {
	n := bulkIndexer.Items()
	if n == 0 {
		return nil
	}
	defer atomic.AddInt64(&i.eventsActive, -int64(n))
	defer atomic.AddInt64(&i.bulkRequests, 1)

	var tx *apm.Transaction
	logger := i.logger
	if i.config.Tracer != nil && i.config.Tracer.Recording() {
		tx = i.config.Tracer.StartTransaction("flush", "output")
		defer tx.End()
		ctx = apm.ContextWithTransaction(ctx, tx)
		tx.Outcome = "success"

		// Add trace IDs to logger, to associate any per-item errors
		// below with the trace.
		for _, field := range apmzap.TraceContext(ctx) {
			logger = logger.With(field)
		}
	}

	resp, err := bulkIndexer.Flush(ctx)
	// Record the bulkIndexer buffer's length as the bytesTotal metric after
	// the request has been flushed.
	if flushed := bulkIndexer.BytesFlushed(); flushed > 0 {
		atomic.AddInt64(&i.bytesTotal, int64(flushed))
	}
	if err != nil {
		atomic.AddInt64(&i.eventsFailed, int64(n))
		logger.With(logp.Error(err)).Error("bulk indexing request failed")
		if tx != nil {
			tx.Outcome = "failure"
			apm.CaptureError(ctx, err).Send()
		}

		var errTooMany errorTooManyRequests
		// 429 may be returned as errors from the bulk indexer.
		if errors.As(err, &errTooMany) {
			atomic.AddInt64(&i.tooManyRequests, int64(n))
		}
		return err
	}
	var eventsFailed, eventsIndexed, tooManyRequests int64
	for _, item := range resp.Items {
		for _, info := range item {
			if info.Error.Type != "" || info.Status > 201 {
				eventsFailed++
				if info.Status == http.StatusTooManyRequests {
					tooManyRequests++
				}
				logger.Errorf(
					"failed to index event (%s): %s",
					info.Error.Type, info.Error.Reason,
				)
			} else {
				eventsIndexed++
			}
		}
	}
	if eventsFailed > 0 {
		atomic.AddInt64(&i.eventsFailed, eventsFailed)
	}
	if eventsIndexed > 0 {
		atomic.AddInt64(&i.eventsIndexed, eventsIndexed)
	}
	if tooManyRequests > 0 {
		atomic.AddInt64(&i.tooManyRequests, tooManyRequests)
	}
	logger.Debugf(
		"bulk request completed: %d indexed, %d failed (%d exceeded capacity)",
		eventsIndexed, eventsFailed, tooManyRequests,
	)
	return nil
}

// runActiveIndexer starts a new active indexer which pulls items from the
// bulkItems channel. The more active indexers there are, the faster events
// will be pulled out of the queue, but also the more likely it is that the
// outgoing Elasticsearch bulk requests are flushed due to the idle timer,
// rather than due to being full.
func (i *Indexer) runActiveIndexer() {
	var closed bool
	var active *bulkIndexer
	var timedFlush uint
	var fullFlush uint
	flushTimer := time.NewTimer(i.config.FlushInterval)
	if !flushTimer.Stop() {
		<-flushTimer.C
	}
	handleBulkItem := func(event elasticsearch.BulkIndexerItem) {
		if active == nil {
			active = <-i.available
			atomic.AddInt64(&i.availableBulkRequests, -1)
			flushTimer.Reset(i.config.FlushInterval)
		}
		if err := active.Add(event); err != nil {
			i.logger.Errorf("failed adding event to bulk indexer: %v", err)
		}
	}
	for !closed {
		select {
		case <-flushTimer.C:
			timedFlush++
			fullFlush = 0
		default:
			// When there's no active indexer and queue utilization is below 5%,
			// reset the flushTimer with IdleInterval so excess active indexers
			// that remain idle can be scaled down.
			if !i.config.Scaling.Disabled && active == nil {
				if i.scalingInformation().activeIndexers > 1 &&
					float64(len(i.bulkItems))/float64(cap(i.bulkItems)) <= 0.05 {
					flushTimer.Reset(i.config.Scaling.IdleInterval)
				}
			}
			select {
			case <-i.closed:
				// Consume whatever bulk items have been buffered,
				// and then flush a last time below.
				for len(i.bulkItems) > 0 {
					select {
					case event := <-i.bulkItems:
						handleBulkItem(event)
					default:
						// Another goroutine took the item.
					}
				}
				closed = true
			case <-flushTimer.C:
				timedFlush++
				fullFlush = 0
			case event := <-i.bulkItems:
				handleBulkItem(event)
				if active.Len() < i.config.FlushBytes {
					continue
				}
				fullFlush++
				timedFlush = 0
				// The active indexer is at or exceeds the configured FlushBytes
				// threshold, so flush it.
				if !flushTimer.Stop() {
					<-flushTimer.C
				}
			}
		}
		if active != nil {
			indexer := active
			active = nil
			i.errgroup.Go(func() error {
				err := i.flush(i.errgroupContext, indexer)
				indexer.Reset()
				i.available <- indexer
				atomic.AddInt64(&i.availableBulkRequests, 1)
				return err
			})
		}
		if i.config.Scaling.Disabled {
			continue
		}
		now := time.Now()
		info := i.scalingInformation()
		if i.maybeScaleDown(now, info, &timedFlush) {
			atomic.AddInt64(&i.activeDestroyed, 1)
			return
		}
		if i.maybeScaleUp(now, info, &fullFlush) {
			atomic.AddInt64(&i.activeCreated, 1)
			i.errgroup.Go(func() error {
				i.runActiveIndexer()
				return nil
			})
		}
	}
	// Decrement the active bulk requests when Indexer is closed.
	for {
		info := i.scalingInformation()
		if i.scalingInfo.CompareAndSwap(info, scalingInfo{
			lastAction:     time.Now(),
			activeIndexers: info.activeIndexers - 1,
		}) {
			return
		}
	}
}

// maybeScaleDown returns true if the caller (assumed to be active indexer) needs
// to be scaled down. It automatically updates the scaling information with a
// decremented `activeBulkRequests` and timestamp of the action when true.
func (i *Indexer) maybeScaleDown(now time.Time, info scalingInfo, timedFlush *uint) bool {
	// Only downscale when there is more than 1 active indexer.
	if info.activeIndexers == 1 {
		return false
	}
	// If the CPU quota changes and there is more than 1 indexer, downscale an
	// active indexer. This downscaling action isn't subject to the downscaling
	// cooldown, since doing so would result in using much more CPU for longer.
	// Loop until the CompareAndSwap operation succeeds (there may be more than)
	// since a single active indexer trying to down scale itself, or the active
	// indexer variable is in check.
	limit := activeLimit()
	for info.activeIndexers > limit {
		// Avoid having more than 1 concurrent downscale, by using a compare
		// and swap operation.
		if i.scalingInfo.CompareAndSwap(info, info.ScaleDown(now)) {
			i.logger.Infof("active indexers (%d) > active limit (%d), scaling down",
				info.activeIndexers, limit,
			)
			return true
		}
		info = i.scalingInformation() // refresh scaling info if CAS failed.
	}
	if *timedFlush < i.config.Scaling.ScaleDown.Threshold {
		return false
	}
	// Reset timedFlush after it has exceeded the threshold
	// it avoids unnecessary precociousness to scale down.
	*timedFlush = 0
	if info.withinCoolDown(i.config.Scaling.ScaleDown.CoolDown, now) {
		return false
	}
	if new := info.ScaleDown(now); i.scalingInfo.CompareAndSwap(info, new) {
		i.logger.Infof("timed flush threshold exceeded, scaling down to: %d", new)
		return true
	}
	return false
}

// maybeScaleUp returns true if the caller (assumed to be active indexer) needs
// to scale up and create another active indexer goroutine. It automatically
// updates the scaling information with an incremented `activeBulkRequests` and
// timestamp of the action when true.
func (i *Indexer) maybeScaleUp(now time.Time, info scalingInfo, fullFlush *uint) bool {
	if *fullFlush < i.config.Scaling.ScaleUp.Threshold {
		return false
	}
	if info.activeIndexers >= activeLimit() {
		return false
	}
	// Reset fullFlush after it has exceeded the threshold
	// it avoids unnecessary precociousness to scale up.
	*fullFlush = 0
	if info.withinCoolDown(i.config.Scaling.ScaleUp.CoolDown, now) {
		return false
	}
	// Avoid having more than 1 concurrent upscale, by using a compare
	// and swap operation.
	if new := info.ScaleUp(now); i.scalingInfo.CompareAndSwap(info, new) {
		i.logger.Infof("full flush threshold exceeded, scaling up to: %d",
			new.activeIndexers,
		)
		return true
	}
	return false
}

func (i *Indexer) scalingInformation() scalingInfo {
	return i.scalingInfo.Load().(scalingInfo)
}

// activeLimit returns the value of GOMAXPROCS / 4. Which should limit the
// maximum number of active indexers to 25% of GOMAXPROCS.
// NOTE: There is also a sweet spot between Config.MaxRequests and the number
// of available indexers, where having N number of available bulk requests per
// active bulk indexer is required for optimal performance.
func activeLimit() int64 {
	if limit := float64(runtime.GOMAXPROCS(0)) / float64(4); limit > 1 {
		return int64(math.RoundToEven(limit))
	}
	return 1
}

var pool sync.Pool

type pooledReader struct {
	jsonw        fastjson.Writer
	reader       *bytes.Reader
	indexBuilder strings.Builder
}

func getPooledReader() *pooledReader {
	if r, ok := pool.Get().(*pooledReader); ok {
		return r
	}
	r := &pooledReader{reader: bytes.NewReader(nil)}
	return r
}

func (r *pooledReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err == io.EOF {
		// Release the reader back into the pool after it has been consumed.
		r.jsonw.Reset()
		r.reader.Reset(nil)
		r.indexBuilder.Reset()
		pool.Put(r)
	}
	return n, err
}

func (r *pooledReader) Seek(offset int64, whence int) (int64, error) {
	return r.reader.Seek(offset, whence)
}

// scalingInfo contains the number of active indexers and the timestamp of the
// latest time a scale action was performed. This structure is used within the
// Indexer to coordinate scale actions with a CompareAndSwap operation.
type scalingInfo struct {
	lastAction     time.Time
	activeIndexers int64
}

func (s scalingInfo) ScaleDown(t time.Time) scalingInfo {
	return scalingInfo{
		lastAction: t, activeIndexers: s.activeIndexers - 1,
	}
}

func (s scalingInfo) ScaleUp(t time.Time) scalingInfo {
	return scalingInfo{
		lastAction: t, activeIndexers: s.activeIndexers + 1,
	}
}

func (s scalingInfo) withinCoolDown(cooldown time.Duration, now time.Time) bool {
	return s.lastAction.Add(cooldown).After(now)
}

// Stats holds bulk indexing statistics.
type Stats struct {
	// Active holds the active number of items waiting in the indexer's queue.
	Active int64

	// Added holds the number of items added to the indexer.
	Added int64

	// BulkRequests holds the number of bulk requests completed.
	BulkRequests int64

	// Failed holds the number of indexing operations that failed.
	Failed int64

	// Indexed holds the number of indexing operations that have completed
	// successfully.
	Indexed int64

	// TooManyRequests holds the number of indexing operations that failed due
	// to Elasticsearch responding with 429 Too many Requests.
	TooManyRequests int64

	// BytesTotal represents the total number of bytes written to the request
	// body that is sent in the outgoing _bulk request to Elasticsearch.
	// The number of bytes written will be smaller when compression is enabled.
	// This implementation differs from the previous number reported by libbeat
	// which counts bytes at the transport level.
	BytesTotal int64

	// AvailableBulkRequests represents the number of bulk indexers
	// available for making bulk index requests.
	AvailableBulkRequests int64

	// IndexersActive represents the number of active bulk indexers that are
	// concurrently processing batches.
	IndexersActive int64

	// IndexersCreated represents the number of times new active indexers were
	// created.
	IndexersCreated int64

	// Downscales represents the number of times an active indexer was destroyed.
	IndexersDestroyed int64
}
