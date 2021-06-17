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
	"fmt"
<<<<<<< HEAD:beater/processors.go
	"time"
=======
>>>>>>> fc605761 (Introduce `apm-server.auth.*` config (#5457)):beater/authprocessor.go

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/ratelimit"
	"github.com/elastic/apm-server/model"
)

const (
	rateLimitTimeout = time.Second
)

// verifyAuthorizedFor is a model.BatchProcessor that checks authorization
// for the agent and service name in the metadata.
func verifyAuthorizedFor(ctx context.Context, meta *model.Metadata) error {
	result, err := authorization.AuthorizedFor(ctx, authorization.Resource{
		AgentName:   meta.Service.Agent.Name,
		ServiceName: meta.Service.Name,
	})
	if err != nil {
		return err
	}
	if result.Authorized {
		return nil
	}
	return fmt.Errorf("%w: %s", authorization.ErrUnauthorized, result.Reason)
<<<<<<< HEAD:beater/processors.go
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
=======
>>>>>>> fc605761 (Introduce `apm-server.auth.*` config (#5457)):beater/authprocessor.go
}
