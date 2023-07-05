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

package beater

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"go.elastic.co/fastjson"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/version"
	"github.com/elastic/go-docappender"
)

const (
	rateLimitTimeout = time.Second
)

// authorizeEventIngestProcessor is a model.BatchProcessor that checks that the
// client is authorized to ingest events for the given agent and service name.
func authorizeEventIngestProcessor(ctx context.Context, batch *modelpb.Batch) error {
	for _, event := range *batch {
		if err := auth.Authorize(ctx, auth.ActionEventIngest, auth.Resource{
			AgentName:   event.Agent.Name,
			ServiceName: event.Service.Name,
		}); err != nil {
			return err
		}
	}
	return nil
}

// rateLimitBatchProcessor is a model.BatchProcessor that rate limits based on
// the batch size. This will be invoked after decoding events, but before sending
// on to the libbeat publisher.
func rateLimitBatchProcessor(ctx context.Context, batch *modelpb.Batch) error {
	if limiter, ok := ratelimit.FromContext(ctx); ok {
		ctx, cancel := context.WithTimeout(ctx, rateLimitTimeout)
		defer cancel()
		if err := limiter.WaitN(ctx, len(*batch)); err != nil {
			return ratelimit.ErrRateLimitExceeded
		}
	}
	return nil
}

// newObserverBatchProcessor returns a model.BatchProcessor that sets
// observer fields from information about the apm-server process.
func newObserverBatchProcessor() modelpb.ProcessBatchFunc {
	hostname, _ := os.Hostname()
	return func(ctx context.Context, b *modelpb.Batch) error {
		for i := range *b {
			if (*b)[i].Observer == nil {
				(*b)[i].Observer = &modelpb.Observer{}
			}
			observer := (*b)[i].Observer
			observer.Hostname = hostname
			observer.Type = "apm-server"
			observer.Version = version.Version
		}
		return nil
	}
}

// TODO remove this once we have added `event.received` to the
// data stream mappings, in 8.10.
func removeEventReceivedBatchProcessor(ctx context.Context, batch *modelpb.Batch) error {
	for _, event := range *batch {
		if event.Event != nil && event.Event.Received != nil {
			event.Event.Received = nil
		}
	}
	return nil
}

func newDocappenderBatchProcessor(a *docappender.Appender) modelpb.ProcessBatchFunc {
	var pool sync.Pool
	pool.New = func() any {
		return &pooledReader{pool: &pool}
	}
	return func(ctx context.Context, b *modelpb.Batch) error {
		for _, event := range *b {
			r := pool.Get().(*pooledReader)
			if err := event.MarshalFastJSON(&r.jsonw); err != nil {
				r.reset()
				return err
			}
			r.indexBuilder.WriteString(event.DataStream.Type)
			r.indexBuilder.WriteByte('-')
			r.indexBuilder.WriteString(event.DataStream.Dataset)
			r.indexBuilder.WriteByte('-')
			r.indexBuilder.WriteString(event.DataStream.Namespace)
			index := r.indexBuilder.String()
			if err := a.Add(ctx, index, r); err != nil {
				r.reset()
				return err
			}
		}
		return nil
	}
}

type pooledReader struct {
	pool         *sync.Pool
	jsonw        fastjson.Writer
	indexBuilder strings.Builder
}

func (r *pooledReader) Read(p []byte) (int, error) {
	panic("should've called WriteTo")
}

func (r *pooledReader) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(r.jsonw.Bytes())
	r.reset()
	return int64(n), err
}

func (r *pooledReader) reset() {
	r.jsonw.Reset()
	r.indexBuilder.Reset()
	r.pool.Put(r)
}
