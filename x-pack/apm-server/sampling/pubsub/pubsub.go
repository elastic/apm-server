// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"go.elastic.co/fastjson"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/elastic/go-elasticsearch/v7/esutil"

	logs "github.com/elastic/apm-server/log"
)

// Pubsub provides a means of publishing and subscribing to sampled trace IDs,
// using Elasticsearch for temporary storage.
//
// An independent process will periodically reap old documents in the index.
type Pubsub struct {
	config  Config
	indexer esutil.BulkIndexer
}

// New returns a new Pubsub which can publish and subscribe sampled trace IDs,
// using Elasticsearch for storage.
//
// Documents are expected to be indexed through a pipeline which sets the
// `event.ingested` timestamp field. Another process will periodically reap
// events older than a configured age.
func New(config Config) (*Pubsub, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid pubsub config")
	}
	if config.Logger == nil {
		config.Logger = logp.NewLogger(logs.Sampling)
	}
	indexer, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client:        config.Client,
		Index:         config.Index,
		FlushInterval: config.FlushInterval,
		OnError: func(ctx context.Context, err error) {
			config.Logger.With(logp.Error(err)).Debug("publishing sampled trace IDs failed")
		},
	})
	if err != nil {
		return nil, err
	}
	return &Pubsub{config: config, indexer: indexer}, nil
}

// PublishSampledTraceIDs bulk indexes traceIDs into Elasticsearch.
func (p *Pubsub) PublishSampledTraceIDs(ctx context.Context, traceID ...string) error {
	for _, id := range traceID {
		var doc traceIDDocument
		doc.Observer.ID = p.config.BeatID
		doc.Trace.ID = id

		var json fastjson.Writer
		if err := doc.MarshalFastJSON(&json); err != nil {
			return err
		}
		if err := p.indexer.Add(ctx, esutil.BulkIndexerItem{
			Action: "index",
			Body:   bytes.NewReader(json.Bytes()),
		}); err != nil {
			return err
		}
	}
	return nil
}

// SubscribeSampledTraceIDs subscribes to new sampled trace IDs, sending them to the
// traceIDs channel.
func (p *Pubsub) SubscribeSampledTraceIDs(ctx context.Context, traceIDs chan<- string) error {
	ticker := time.NewTicker(p.config.SearchInterval)
	defer ticker.Stop()

	// NOTE(axw) we should use the Changes API when it is implemented:
	// https://github.com/elastic/elasticsearch/issues/1242

	var lastSeqNo int64 = 0
	var lastPrimaryTerm int64 = -1
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		for {
			// Keep searching until there are no more new trace IDs.
			n, err := p.searchTraceIDs(ctx, traceIDs, &lastSeqNo, &lastPrimaryTerm)
			if err != nil {
				// Errors may occur due to rate limiting, or while the index is
				// still being created, so just log and continue.
				p.config.Logger.With(logp.Error(err)).Debug("error searching for trace IDs")
				break
			}
			if n == 0 {
				// No more results, go back to sleep.
				break
			}
		}
	}
}

// searchTraceIDs searches for new sampled trace IDs (after lastPrimaryTerm and lastSeqNo),
// sending them to the out channel and returning the number of trace IDs sent.
func (p *Pubsub) searchTraceIDs(ctx context.Context, out chan<- string, lastSeqNo, lastPrimaryTerm *int64) (int, error) {
	searchBody := map[string]interface{}{
		"size":                1000,
		"seq_no_primary_term": true,
		"sort":                []interface{}{map[string]interface{}{"_seq_no": "asc"}},
		"search_after":        []interface{}{*lastSeqNo - 1},
		"query": map[string]interface{}{
			// Filter out local observations.
			"bool": map[string]interface{}{
				"must_not": map[string]interface{}{
					"term": map[string]interface{}{
						"observer.id": map[string]interface{}{
							"value": p.config.BeatID,
						},
					},
				},
			},
		},
	}

	req := esapi.SearchRequest{
		Index: []string{p.config.Index},
		Body:  esutil.NewJSONReader(searchBody),
	}
	resp, err := req.Do(ctx, p.config.Client)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return 0, nil
		}
		message, _ := ioutil.ReadAll(resp.Body)
		return 0, fmt.Errorf("search request failed: %s", message)
	}

	var result struct {
		Hits struct {
			Hits []struct {
				SeqNo       int64           `json:"_seq_no,omitempty"`
				PrimaryTerm int64           `json:"_primary_term,omitempty"`
				Source      traceIDDocument `json:"_source"`
			}
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	if len(result.Hits.Hits) == 0 {
		return 0, nil
	}

	var n int
	maxPrimaryTerm := *lastPrimaryTerm
	for _, hit := range result.Hits.Hits {
		if hit.SeqNo < *lastSeqNo || (hit.SeqNo == *lastSeqNo && hit.PrimaryTerm <= *lastPrimaryTerm) {
			continue
		}
		select {
		case <-ctx.Done():
			return n, ctx.Err()
		case out <- hit.Source.Trace.ID:
			n++
		}
		if hit.PrimaryTerm > maxPrimaryTerm {
			maxPrimaryTerm = hit.PrimaryTerm
		}
	}
	// we sort by hit.SeqNo, but not _primary_term (you can't?)
	*lastSeqNo = result.Hits.Hits[len(result.Hits.Hits)-1].SeqNo
	*lastPrimaryTerm = maxPrimaryTerm
	return n, nil
}

type traceIDDocument struct {
	// Observer identifies the entity (typically an APM Server) that observed
	// and indexed the/ trace ID document. This can be used to filter out local
	// observations.
	Observer struct {
		// ID holds the unique ID of the observer.
		ID string `json:"id"`
	} `json:"observer"`

	// Trace identifies a trace.
	Trace struct {
		// ID holds the unique ID of the trace.
		ID string `json:"id"`
	} `json:"trace"`
}

func (d *traceIDDocument) MarshalFastJSON(w *fastjson.Writer) error {
	w.RawString(`{"observer":{"id":`)
	w.String(d.Observer.ID)
	w.RawString(`},`)
	w.RawString(`"trace":{"id":`)
	w.String(d.Trace.ID)
	w.RawString(`}}`)
	return nil
}
