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
	"net/http"
	"time"

	"go.elastic.co/apm"

	"github.com/elastic/beats/v7/libbeat/logp"
)

// waitReady waits for preconditions to be satisfied, by calling check
// in a loop every interval until ctx is cancelled or check returns nil.
func waitReady(
	ctx context.Context,
	interval time.Duration,
	tracer *apm.Tracer,
	logger *logp.Logger,
	check func(context.Context) error,
) error {
	logger.Info("waiting for preconditions")
	tx := tracer.StartTransaction("wait_for_preconditions", "init")
	ctx = apm.ContextWithTransaction(ctx, tx)
	var ticker *time.Ticker
	for {
		if ticker == nil {
			// We start the ticker on the first iteration, rather than
			// before the loop, so we don't have to wait for a tick
			// (5 seconds by default) before peforming the first check.
			ticker = time.NewTicker(interval)
			defer ticker.Stop()
		} else {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
		}
		if err := check(ctx); err != nil {
			logger.Errorf("error checking preconditions: %s", err)
			continue
		}
		return nil
	}
}

// waitReadyRoundTripper wraps a *net/http.Transport, ensuring the server's
// indexing preconditions have been satisfied by waiting for "ready" channel
// to be signalled, prior to allowing any requests through.
//
// This is used to prevent elasticsearch clients from proceeding with requests
// until the APM integration is installed to ensure we don't index any documents
// prior to the data stream index templates being ready.
type waitReadyRoundTripper struct {
	*http.Transport
	ready <-chan struct{}
}

func (c *waitReadyRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	select {
	case <-c.ready:
	case <-r.Context().Done():
		return nil, r.Context().Err()
	}
	return c.Transport.RoundTrip(r)
}
