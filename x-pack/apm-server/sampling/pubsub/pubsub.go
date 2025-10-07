// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.elastic.co/fastjson"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-docappender/v2"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/esutil"

	"github.com/elastic/apm-data/model/modeljson"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/logs"
)

// ErrClosed may be returned by Pubsub methods after the Close method is called.
var ErrClosed = errors.New("pubsub closed")

var (
	errIndexNotFound   = errors.New("index not found")
	errTooManyRequests = errors.New("429")
)

// Pubsub provides a means of publishing and subscribing to sampled trace IDs,
// using Elasticsearch for temporary storage.
//
// An independent process will periodically reap old documents in the index.
type Pubsub struct {
	config Config
}

// New returns a new Pubsub which can publish and subscribe sampled trace IDs,
// using Elasticsearch for storage. The Pubsub.Close method must be called when
// it is no longer needed.
//
// Documents are expected to be indexed through a pipeline which sets the
// `event.ingested` timestamp field. Another process will periodically reap
// events older than a configured age.
func New(config Config) (*Pubsub, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pubsub config: %w", err)
	}
	if config.Logger == nil {
		config.Logger = logp.NewLogger(logs.Sampling)
	}
	return &Pubsub{config: config}, nil
}

// PublishSampledTraceIDs receives trace IDs from the traceIDs channel,
// indexing them into Elasticsearch. PublishSampledTraceIDs returns when
// ctx is canceled, or traceIDs is closed.
func (p *Pubsub) PublishSampledTraceIDs(ctx context.Context, traceIDs <-chan string) error {
	appender, err := docappender.New(p.config.Client, docappender.Config{
<<<<<<< HEAD
		CompressionLevel:   p.config.CompressionLevel,
		FlushInterval:      p.config.FlushInterval,
		DocumentBufferSize: 100, // Reduce memory footprint
=======
		CompressionLevel:     p.config.CompressionLevel,
		FlushInterval:        p.config.FlushInterval,
		DocumentBufferSize:   100, // Reduce memory footprint
		IncludeSourceOnError: docappender.False,
		TracerProvider:       tracenoop.NewTracerProvider(),
		MeterProvider:        metricnoop.NewMeterProvider(),
>>>>>>> dece0323 (fix: pass noop otel providers to pubsub docappender (#18891))
		// Disable autoscaling for the TBS sampled traces published documents.
		Scaling: docappender.ScalingConfig{Disabled: true},
		Logger:  zap.New(p.config.Logger.Core(), zap.WithCaller(true)),
	})
	if err != nil {
		return err
	}

	result := p.indexSampledTraceIDs(ctx, traceIDs, appender)
	ctx, cancel := context.WithTimeout(context.Background(), p.config.FlushInterval)
	defer cancel()
	closeErr := appender.Close(ctx)
	return errors.Join(result, closeErr)
}

func (p *Pubsub) indexSampledTraceIDs(ctx context.Context, traceIDs <-chan string, appender *docappender.Appender) error {
	index := p.config.DataStream.String()
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != context.Canceled {
				return err
			}
			return nil
		case id, ok := <-traceIDs:
			if !ok {
				return nil
			}
			var w fastjson.Writer
			doc := modelpb.APMEvent{
				Timestamp: modelpb.FromTime(time.Now()),
				DataStream: &modelpb.DataStream{
					Type:      p.config.DataStream.Type,
					Dataset:   p.config.DataStream.Dataset,
					Namespace: p.config.DataStream.Namespace,
				},
				Agent: &modelpb.Agent{EphemeralId: p.config.ServerID},
				Trace: &modelpb.Trace{Id: id},
			}
			err := modeljson.MarshalAPMEvent(&doc, &w)
			if err != nil {
				p.config.Logger.With(
					logp.Error(err),
					logp.Reflect("event", &doc),
				).Error("failed to encode sampled trace ID document")
				return err
			}
			data := w.Bytes()
			if err := appender.Add(ctx, index, bytes.NewReader(data)); err != nil {
				p.config.Logger.With(
					logp.Error(err),
					logp.Reflect("event", &doc),
				).Error("failed to index sampled trace ID document")
				return err
			}
		}
	}
}

// SubscribeSampledTraceIDs subscribes to sampled trace IDs after the given position,
// sending them to the traceIDs channel, and sending the most recently observed position
// (on change) to the positions channel.
func (p *Pubsub) SubscribeSampledTraceIDs(
	ctx context.Context,
	pos SubscriberPosition,
	traceIDs chan<- string,
	positions chan<- SubscriberPosition,
) error {
	ticker := time.NewTicker(p.config.SearchInterval)
	defer ticker.Stop()

	// Only send positions on change.
	var positionsOut chan<- SubscriberPosition
	positionsOut = positions

	// Copy pos because it may be mutated by p.searchTraceIDs.
	pos = copyPosition(pos)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case positionsOut <- pos:
			// Copy pos because it may be mutated by p.searchTraceIDs.
			pos = copyPosition(pos)
			positionsOut = nil
		case <-ticker.C:
			changed, err := p.searchTraceIDs(ctx, traceIDs, pos.observedSeqnos)
			if err != nil {
				logger := p.config.Logger.With(logp.Error(err)).With(logp.Reflect("position", pos))
				switch {
				case errors.Is(err, context.Canceled):
					// Ignore shutdown errors
				case errors.Is(err, errTooManyRequests):
					logger.Warn("error searching for trace IDs")
				default:
					logger.Error("error searching for trace IDs")
				}
				continue
			}
			if changed {
				positionsOut = positions
			}
		}
	}
}

// searchTraceIDs searches the configured data stream for new sampled trace IDs, sending them to the out channel.
//
// searchTraceIDs works by fetching the global checkpoint for each index backing the data stream, and comparing
// this to the most recently observed sequence number for the indices. If the global checkpoint is greater, then
// we search through every document with a sequence number greater than the most recently observed, and less than
// or equal to the global checkpoint.
//
// Immediately after observing an updated global checkpoint we will force-refresh indices to ensure all documents
// up to the global checkpoint are visible in proceeding searches.
func (p *Pubsub) searchTraceIDs(ctx context.Context, out chan<- string, observedSeqnos map[string]int64) (bool, error) {
	globalCheckpoints, err := getGlobalCheckpoints(ctx, p.config.Client, p.config.DataStream.String())
	if err != nil {
		return false, err
	}

	// Remove old indices from the observed _seq_no map.
	for index := range observedSeqnos {
		if _, ok := globalCheckpoints[index]; !ok {
			delete(observedSeqnos, index)
		}
	}

	// Force-refresh the indices with updated global checkpoints.
	indices := make([]string, 0, len(globalCheckpoints))
	for index, globalCheckpoint := range globalCheckpoints {
		observedSeqno, ok := observedSeqnos[index]
		if ok && globalCheckpoint <= observedSeqno {
			delete(globalCheckpoints, index)
			continue
		}
		indices = append(indices, index)
	}
	if err := p.refreshIndices(ctx, indices); err != nil {
		return false, err
	}

	var changed bool
	var observedSeqnosMu sync.Mutex
	g, ctx := errgroup.WithContext(ctx)
	for _, index := range indices {
		globalCheckpoint := globalCheckpoints[index]
		observedSeqno, ok := observedSeqnos[index]
		if !ok {
			observedSeqno = -1
		}
		index := index // copy for closure
		g.Go(func() error {
			maxSeqno, err := p.searchIndexTraceIDs(ctx, out, index, observedSeqno, globalCheckpoint)
			if err != nil {
				return err
			}
			if maxSeqno > observedSeqno {
				observedSeqnosMu.Lock()
				observedSeqno = maxSeqno
				observedSeqnos[index] = observedSeqno
				changed = true
				observedSeqnosMu.Unlock()
			}
			return nil
		})
	}
	return changed, g.Wait()
}

func (p *Pubsub) refreshIndices(ctx context.Context, indices []string) error {
	if len(indices) == 0 {
		return nil
	}
	ignoreUnavailable := true
	resp, err := esapi.IndicesRefreshRequest{
		Index:             indices,
		IgnoreUnavailable: &ignoreUnavailable,
	}.Do(ctx, p.config.Client)
	if err != nil {
		return fmt.Errorf("index refresh request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		message, _ := io.ReadAll(resp.Body)
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			return fmt.Errorf("index refresh request failed with status code %w: %s", errTooManyRequests, message)
		}
		return fmt.Errorf("index refresh request failed with status code %d: %s", resp.StatusCode, message)
	}
	return nil
}

// searchIndexTraceIDs searches index sampled trace IDs, whose documents have a _seq_no
// greater than minSeqno and less than or equal to maxSeqno, and returns the greatest
// observed _seq_no. Sampled trace IDs are sent to out.
func (p *Pubsub) searchIndexTraceIDs(ctx context.Context, out chan<- string, index string, minSeqno, maxSeqno int64) (int64, error) {
	var maxObservedSeqno int64 = -1
	for maxObservedSeqno < maxSeqno {
		// Include only documents after the old global checkpoint,
		// and up to and including the new global checkpoint.
		filters := []map[string]interface{}{{
			"range": map[string]interface{}{
				"_seq_no": map[string]interface{}{
					"lte": maxSeqno,
				},
			},
		}}
		if minSeqno >= 0 {
			filters = append(filters, map[string]interface{}{
				"range": map[string]interface{}{
					"_seq_no": map[string]interface{}{
						"gt": minSeqno,
					},
				},
			})
		}

		searchBody := map[string]interface{}{
			"size":                1000,
			"sort":                []interface{}{map[string]interface{}{"_seq_no": "asc"}},
			"seq_no_primary_term": true,
			"track_total_hits":    false,
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					// Filter out local observations.
					"must_not": map[string]interface{}{
						"term": map[string]interface{}{
							"agent.ephemeral_id": map[string]interface{}{
								"value": p.config.ServerID,
							},
						},
					},
					"filter": filters,
				},
			},
		}

		var result struct {
			Hits struct {
				Hits []struct {
					Seqno  int64           `json:"_seq_no"`
					Source traceIDDocument `json:"_source"`
					Sort   []interface{}   `json:"sort"`
				}
			}
		}
		if err := p.doSearchRequest(ctx, index, esutil.NewJSONReader(searchBody), &result); err != nil {
			if err == errIndexNotFound {
				// Index was deleted.
				break
			}
			return -1, err
		}
		if len(result.Hits.Hits) == 0 {
			break
		}
		for _, hit := range result.Hits.Hits {
			select {
			case <-ctx.Done():
				return -1, ctx.Err()
			case out <- hit.Source.Trace.ID:
			}
		}
		maxObservedSeqno = result.Hits.Hits[len(result.Hits.Hits)-1].Seqno
		minSeqno = maxObservedSeqno
	}
	return maxObservedSeqno, nil
}

func (p *Pubsub) doSearchRequest(ctx context.Context, index string, body io.Reader, out interface{}) error {
	resp, err := esapi.SearchRequest{
		Index: []string{index},
		Body:  body,
	}.Do(ctx, p.config.Client)
	if err != nil {
		return fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		if resp.StatusCode == http.StatusNotFound {
			return errIndexNotFound
		}
		message, _ := io.ReadAll(resp.Body)
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			return fmt.Errorf("search request failed with status code %w: %s", errTooManyRequests, message)
		}
		return fmt.Errorf("search request failed with status code %d: %s", resp.StatusCode, message)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to parse search response: %w", err)
	}
	return nil
}

type traceIDDocument struct {
	// Agent identifies the entity (typically an APM Server) that observed
	// and indexed the sampled trace ID document. This can be used to filter
	// out local observations.
	Agent struct {
		// EphemeralID holds the unique ID of the agent.
		EphemeralID string `json:"ephemeral_id"`
	} `json:"agent"`

	// Trace identifies a trace.
	Trace struct {
		// ID holds the unique ID of the trace.
		ID string `json:"id"`
	} `json:"trace"`
}
