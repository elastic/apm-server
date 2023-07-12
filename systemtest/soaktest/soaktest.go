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
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-perf/loadgen"
	loadgencfg "github.com/elastic/apm-perf/loadgen/config"
)

func RunBlocking(ctx context.Context) error {
	limiter := loadgen.GetNewLimiter(loadgencfg.Config.EventRate.Burst, loadgencfg.Config.EventRate.Interval)
	g, gCtx := errgroup.WithContext(ctx)

	// Create a Rand with the same seed for each agent, so we randomise their IDs consistently.
	//rand :=
	var rngseed int64
	err := binary.Read(cryptorand.Reader, binary.LittleEndian, &rngseed)
	if err != nil {
		return fmt.Errorf("failed to generate seed for math/rand: %w", err)
	}

	for i := 0; i < soakConfig.AgentsReplicas; i++ {
		for _, expr := range []string{`go*.ndjson`, `nodejs*.ndjson`, `python*.ndjson`, `ruby*.ndjson`} {
			expr := expr
			g.Go(func() error {
				rng := rand.New(rand.NewSource(rngseed))
				return runAgent(gCtx, expr, limiter, rng, loadgencfg.Config.Headers)
			})
		}
	}

	return g.Wait()
}

func runAgent(ctx context.Context, expr string, limiter *rate.Limiter, rng *rand.Rand, headers map[string]string) error {
	handler, err := loadgen.NewEventHandler(loadgen.EventHandlerParams{
		Path:                      expr,
		URL:                       loadgencfg.Config.ServerURL.String(),
		Token:                     loadgencfg.Config.SecretToken,
		APIKey:                    loadgencfg.Config.APIKey,
		Limiter:                   limiter,
		Rand:                      rng,
		RewriteIDs:                loadgencfg.Config.RewriteIDs,
		RewriteServiceNames:       loadgencfg.Config.RewriteServiceNames,
		RewriteServiceNodeNames:   loadgencfg.Config.RewriteServiceNodeNames,
		RewriteServiceTargetNames: loadgencfg.Config.RewriteServiceTargetNames,
		RewriteSpanNames:          loadgencfg.Config.RewriteSpanNames,
		RewriteTransactionNames:   loadgencfg.Config.RewriteTransactionNames,
		RewriteTransactionTypes:   loadgencfg.Config.RewriteTransactionTypes,
		RewriteTimestamps:         loadgencfg.Config.RewriteTimestamps,
		Headers:                   headers,
	})
	if err != nil {
		return err
	}

	if err := handler.SendBatchesInLoop(ctx); err != nil {
		return err
	}

	return nil
}
