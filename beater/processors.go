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
	"time"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/ratelimit"
	"github.com/elastic/apm-server/model"
)

const (
	rateLimitTimeout = time.Second
)

// authorizeEventIngest is a model.BatchProcessor that checks that the client
// is authorized to ingest events for the agent and service name in metadata.
func authorizeEventIngest(ctx context.Context, meta *model.Metadata) error {
	return auth.Authorize(ctx, auth.ActionEventIngest, auth.Resource{
		AgentName:   meta.Service.Agent.Name,
		ServiceName: meta.Service.Name,
	})
}

// rateLimitBatchProcessor is a model.BatchProcessor that rate limits based on
// the batch size. This will be invoked after decoding events, but before sending
// on to the libbeat publisher.
func rateLimitBatchProcessor(ctx context.Context, batch *model.Batch) error {
	if limiter, ok := ratelimit.FromContext(ctx); ok {
		ctx, cancel := context.WithTimeout(ctx, rateLimitTimeout)
		defer cancel()
		if err := limiter.WaitN(ctx, batch.Len()); err != nil {
			return ratelimit.ErrRateLimitExceeded
		}
	}
	return nil
}
