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

package soaktest

import (
	"context"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-server/systemtest/loadgen"
	loadgencfg "github.com/elastic/apm-server/systemtest/loadgen/config"
)

func RunBlocking(ctx context.Context) error {
	limiter := loadgen.GetNewLimiter(loadgencfg.Config.MaxEPM)
	g, gCtx := errgroup.WithContext(ctx)

	for i := 0; i < soakConfig.AgentsReplicas; i++ {
		for _, expr := range []string{`go*.ndjson`, `nodejs*.ndjson`, `python*.ndjson`, `ruby*.ndjson`} {
			expr := expr
			g.Go(func() error {
				return runAgent(gCtx, expr, limiter)
			})
		}
	}

	return g.Wait()
}

func runAgent(ctx context.Context, expr string, limiter *rate.Limiter) error {
	handler, err := loadgen.NewEventHandler(loadgen.EventHandlerParams{
		Path:              expr,
		URL:               loadgencfg.Config.ServerURL.String(),
		Token:             loadgencfg.Config.SecretToken,
		APIKey:            loadgencfg.Config.APIKey,
		Limiter:           limiter,
		RewriteTimestamps: loadgencfg.Config.RewriteTimestamps,
	})
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_, err = handler.SendBatches(ctx)
			if err != nil {
				return err
			}
		}
	}
}
