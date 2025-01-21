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

package gen

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/elastic/apm-perf/pkg/telemetrygen"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
)

type Generator struct {
	Logger       *zap.Logger
	APMAPIKey    string
	APMServerURL string
	EventRate    string
}

func New(url, apikey string) *Generator {
	return &Generator{
		Logger:       zap.NewNop(),
		APMAPIKey:    apikey,
		APMServerURL: url,
		EventRate:    "1000/s",
	}
}

// RunBlocking runs the underlying generator in blocking mode.
func (g *Generator) RunBlocking(ctx context.Context) error {
	cfg := telemetrygen.DefaultConfig()
	cfg.APIKey = g.APMAPIKey

	u, err := url.Parse(g.APMServerURL)
	if err != nil {
		return fmt.Errorf("cannot parse APM server URL: %w", err)
	}
	cfg.ServerURL = u

	cfg.EventRate.Set(g.EventRate)
	gen, err := telemetrygen.New(cfg)
	if err != nil {
		return fmt.Errorf("cannot create telemetrygen Generator: %w", err)
	}

	g.Logger.Info("ingest data")
	gen.Logger = g.Logger
	return gen.RunBlocking(ctx)
}

// RunBlockingWait runs the underlying generator in blocking mode and waits until the
// document count retrieved through ecc matches expected or timeout. It supports
// specifying previous document count to offset the expectation based on a previous state.
// expected and previous must be maps of <datastream name, document count>.
// A document count value lower than 0 will signal this function to only check for
// data stream presence ignoring the document count.
func (g *Generator) RunBlockingWait(ctx context.Context, ecc *esclient.Client, expected, previous map[string]int, timeout time.Duration) error {
	if err := g.RunBlocking(ctx); err != nil {
		return fmt.Errorf("cannot run generator: %w", err)
	}

	// this function checks that expected docs count is reached,
	// accounting for any previous state.
	checkDocsCount := func(docsCount map[string]int) bool {
		equal := false
		for ds, c := range docsCount {
			if e, ok := expected[ds]; ok && e >= 0 {
				got := c - previous[ds]
				equal = (e == got)
			}
		}
		return equal
	}
	// this function checks that all expected data streams are
	// present in the returned response, ignoring the doc counts.
	checkDSPresence := func(docsCount map[string]int) bool {
		present := false
		for ds := range docsCount {
			if _, ok := expected[ds]; ok {
				present = true
			} else {
				present = false
			}
		}
		return present
	}
	prevdocs := map[string]int{}
	equaltoprevdocs := 0
	// this function checks that all returned data streams doc counts
	// stay still for some iterations. This forces a 15 seconds longer wait
	// but ensures that aggredation data streams have stabilized before
	// allowing the code to proceed. This should prevent situation where
	// aggregation are still running when expected data streams reached
	// their expected doc count.
	checkAggregationDocs := func(prevdocs, docsCount map[string]int) bool {
		if equaltoprevdocs == 3 {
			return true
		}
		if fmt.Sprint(prevdocs) == fmt.Sprint(docsCount) {
			equaltoprevdocs += 1
		}
		return false
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		select {
		case <-tctx.Done():
			return nil
		case <-ticker.C:
			docsCount, err := ecc.ApmDocCount(ctx)
			if err != nil {
				return fmt.Errorf("cannot retrieve APM doc count: %w", err)
			}
			// to confirm ingestion was correctly completed we:
			// - wait until docs counts reach the expected values
			// - wait until all expected data streams are present, ignoring the doc counts
			// - wait for ingestion in all data stream to stabilize for 15s
			if checkDocsCount(docsCount) &&
				checkDSPresence(docsCount) &&
				checkAggregationDocs(prevdocs, docsCount) {
				return nil
			}
			prevdocs = docsCount
		}
	}
}
