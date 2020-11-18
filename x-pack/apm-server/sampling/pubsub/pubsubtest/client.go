// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pubsubtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/elastic/apm-server/elasticsearch"
)

// Publisher is an interface to pass to Client that responds to publish
// requests, consuming a trace ID sent by the requester.
type Publisher interface {
	Publish(ctx context.Context, traceID string) error
}

// PublisherChan is a Publisher implemented as a channel.
type PublisherChan chan<- string

// Publish waits for traceID to be sent on c, or for ctx to be done.
func (c PublisherChan) Publish(ctx context.Context, traceID string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c <- traceID:
		return nil
	}
}

// PublisherFunc is a Publisher implemented as a function.
type PublisherFunc func(context.Context, string) error

// Publish calls f(ctx, traceID).
func (f PublisherFunc) Publish(ctx context.Context, traceID string) error {
	return f(ctx, traceID)
}

// Subscriber is an interface to pass to Client that responds to subscribe
// requests, returning a trace ID to send back to the requester.
type Subscriber interface {
	Subscribe(ctx context.Context) (traceID string, err error)
}

// SubscriberChan is a Subscriber implemented as a channel.
type SubscriberChan <-chan string

// Subscribe waits for a trace ID to be received on c, or for ctx to be done.
func (c SubscriberChan) Subscribe(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case traceID, ok := <-c:
		if !ok {
			return "", errors.New("channel closed")
		}
		return traceID, nil
	}
}

// SubscriberFunc is a Subscriber implemented as a function.
type SubscriberFunc func(ctx context.Context) (string, error)

// Subscribe calls f(ctx).
func (f SubscriberFunc) Subscribe(ctx context.Context) (string, error) {
	return f(ctx)
}

// Client returns a new elasticsearch.Client, suitable for use with pubsub,
// that responds to publish requests by calling pub (if non-nil) and subscribe
// requests by calling sub (if non-nil). If either function is nil, then the
// respective operation will be a no-op.
func Client(pub Publisher, sub Subscriber) elasticsearch.Client {
	client, err := elasticsearch.NewVersionedClient(
		"", // API Key
		"", // user
		"", // password,
		[]string{"testing.invalid"}, // addresses
		nil, // headers
		&channelClientRoundTripper{pub: pub, sub: sub},
	)
	if err != nil {
		panic(err)
	}
	return client
}

type channelClientRoundTripper struct {
	pub   Publisher
	sub   Subscriber
	seqno int64
}

func (rt *channelClientRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	type traceIDDocument struct {
		Observer struct {
			ID string `json:"id"`
		} `json:"observer"`

		Trace struct {
			ID string `json:"id"`
		} `json:"trace"`
	}
	type traceIDDocumentHit struct {
		SeqNo       int64           `json:"_seq_no,omitempty"`
		PrimaryTerm int64           `json:"_primary_term,omitempty"`
		Source      traceIDDocument `json:"_source"`
	}

	recorder := httptest.NewRecorder()
	switch r.Method {
	case "GET":
		// Subscribe
		var result struct {
			Hits struct {
				Hits []traceIDDocumentHit
			}
		}
		if rt.sub != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 50*time.Millisecond)
			defer cancel()
			for {
				var traceID string
				err := ctx.Err()
				if err == nil {
					traceID, err = rt.sub.Subscribe(ctx)
				}
				if err == context.DeadlineExceeded {
					break
				} else if err != nil {
					return nil, err
				}
				rt.seqno++
				hit := traceIDDocumentHit{SeqNo: rt.seqno, PrimaryTerm: 1}
				hit.Source.Trace.ID = traceID
				hit.Source.Observer.ID = "👀"
				result.Hits.Hits = append(result.Hits.Hits, hit)
			}
		}
		if err := json.NewEncoder(recorder).Encode(result); err != nil {
			return nil, err
		}
		recorder.Flush()
	case "POST":
		// Publish
		var results []map[string]elasticsearch.BulkIndexerResponseItem
		dec := json.NewDecoder(r.Body)
		for {
			var m map[string]interface{}
			if err := dec.Decode(&m); err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			var action string
			for action = range m {
			}
			if action != "index" {
				// We only ever issue index operations.
				panic("unexpected action: " + action)
			}
			var doc traceIDDocument
			if err := dec.Decode(&doc); err != nil {
				return nil, err
			}
			if doc.Trace.ID == "" {
				panic("empty trace ID")
			}
			if rt.pub != nil {
				if err := rt.pub.Publish(r.Context(), doc.Trace.ID); err != nil {
					return nil, err
				}
			}
			result := elasticsearch.BulkIndexerResponseItem{Status: 200}
			results = append(results, map[string]elasticsearch.BulkIndexerResponseItem{action: result})
		}
		if err := json.NewEncoder(recorder).Encode(results); err != nil {
			return nil, err
		}
	}
	return recorder.Result(), nil
}
