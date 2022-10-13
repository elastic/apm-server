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
	"net/http"
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
	activeBulkRequests    int64

	config    Config
	logger    *logp.Logger
	available chan *bulkIndexer
	g         errgroup.Group

	mu        sync.RWMutex
	closing   bool
	closed    chan struct{}
	bulkItems chan elasticsearch.BulkIndexerItem
}

// Config holds configuration for Indexer.
type Config struct {
	// CompressionLevel holds the gzip compression level, from 0 (gzip.NoCompression)
	// to 9 (gzip.BestCompression). Higher values provide greater compression, at a
	// greater cost of CPU. The special value -1 (gzip.DefaultCompression) selects the
	// default compression level.
	CompressionLevel int

	// MaxRequests holds the maximum number of bulk index requests to execute concurrently.
	// The maximum memory usage of Indexer is thus approximately MaxRequests*FlushBytes.
	//
	// If MaxRequests is less than or equal to zero, the default of 25 will be used.
	MaxRequests int

	// FlushBytes holds the flush threshold in bytes. If Compression is enabled,
	// The number of events that can be buffered will be greater.
	//
	// If FlushBytes is zero, the default of 2MB will be used.
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
	//
	// If Tracer is nil, requests will not be traced.
	Tracer *apm.Tracer
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
		cfg.MaxRequests = 25
	}
	if cfg.FlushBytes <= 0 {
		cfg.FlushBytes = 2 * 1024 * 1024
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 30 * time.Second
	}
	if cfg.EventBufferSize <= 0 {
		cfg.EventBufferSize = 64
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
	indexer.g.Go(func() error {
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
	if !i.closing {
		i.closing = true
		close(i.bulkItems)
		// Close i.closed when ctx is cancelled,
		// unblocking any ongoing flush attempts.
		done := make(chan struct{})
		defer close(done)
		go func() {
			defer close(i.closed)
			select {
			case <-done:
			case <-ctx.Done():
			}
		}()
	}
	return i.g.Wait()
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
		ActiveBulkRequests:    atomic.LoadInt64(&i.activeBulkRequests),
	}
}

// ProcessBatch creates a document for each event in batch, and adds them to the
// Elasticsearch bulk indexer.
//
// If the indexer has been closed, ProcessBatch returns ErrClosed.
func (i *Indexer) ProcessBatch(ctx context.Context, batch *model.Batch) error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.closing {
		return ErrClosed
	}
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

	select {
	// Send the BulkIndexerItem to the internal channel, allowing individual
	// events to be processed by an active bulk indexer in a dedicated goroutine,
	// which in turn speeds up event processing.
	case i.bulkItems <- elasticsearch.BulkIndexerItem{
		Index:  r.indexBuilder.String(),
		Action: "create",
		Body:   r,
	}:
	case <-ctx.Done():
		return ctx.Err()
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

func (i *Indexer) flushIndexer(ctx context.Context, bulkIndexer *bulkIndexer) error {
	// Create a child context which is cancelled when the context passed to i.Close is cancelled.
	flushed := make(chan struct{})
	defer close(flushed)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		select {
		case <-i.closed:
		case <-flushed:
		}
	}()

	err := i.flush(ctx, bulkIndexer)
	bulkIndexer.Reset()
	i.available <- bulkIndexer
	atomic.AddInt64(&i.availableBulkRequests, 1)
	return err
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
	atomic.AddInt64(&i.activeBulkRequests, 1)
	defer atomic.AddInt64(&i.activeBulkRequests, -1)
	var closed bool
	var active *bulkIndexer
	flushTimer := time.NewTimer(i.config.FlushInterval)
	if !flushTimer.Stop() {
		<-flushTimer.C
	}
	for !closed {
		select {
		case <-flushTimer.C:
		default:
			select {
			case <-flushTimer.C:
			case event, ok := <-i.bulkItems:
				if !ok {
					closed = true
					break // Flush a last time below, if there's an active indexer
				}
				if active == nil {
					active = <-i.available
					atomic.AddInt64(&i.availableBulkRequests, -1)
					flushTimer.Reset(i.config.FlushInterval)
				}
				if err := active.Add(event); err != nil {
					i.logger.Errorf("failed adding event to bulk indexer: %v", err)
				}
				// Flush the active indexer when it's at or exceeds the configured
				// FlushBytes threshold.
				if active.Len() >= i.config.FlushBytes {
					if !flushTimer.Stop() {
						<-flushTimer.C
					}
				} else {
					continue
				}
			}
		}
		if active != nil {
			indexer := active
			active = nil
			i.g.Go(func() error {
				return i.flushIndexer(context.Background(), indexer)
			})
		}
	}
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

	// ActiveBulkRequests represents the number of active bulk indexers that are
	// concurrently processing batches.
	ActiveBulkRequests int64
}
