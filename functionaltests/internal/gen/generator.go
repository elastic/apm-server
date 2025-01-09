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
func (g *Generator) RunBlockingWait(ctx context.Context, ecc *esclient.Client, expected, previous map[string]int, timeout time.Duration) error {
	if err := g.RunBlocking(ctx); err != nil {
		return fmt.Errorf("cannot run generator: %w", err)
	}

	checkDocsCount := func(docsCount map[string]int) bool {
		equal := false
		for ds, c := range docsCount {
			if e, ok := expected[ds]; ok {
				got := c - previous[ds]
				equal = (e == got)
			}
		}
		return equal
	}
	getDocsCount := func() (map[string]int, error) {
		res := map[string]int{}
		docCounts, err := ecc.ApmDocCount(ctx)
		if err != nil {
			return res, fmt.Errorf("cannot retrieve APM doc count: %w", err)
		}
		for _, dc := range docCounts {
			res[dc.Datastream] = dc.Count
		}
		return res, nil
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
			docsCount, err := getDocsCount()
			if err != nil {
				return err
			}
			if checkDocsCount(docsCount) {
				return nil
			}
		}
	}
}
