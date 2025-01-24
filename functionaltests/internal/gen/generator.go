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

	"go.uber.org/zap"

	"github.com/elastic/apm-perf/pkg/telemetrygen"
	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
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
// cluster Integrations Server has been restarted.
// Restarting APM Server ensures all data, including aggregations, in flushed before
// shutdown, ensuring ingestion and 1m aggregations to be completed.
func (g *Generator) RunBlockingWait(ctx context.Context, c *ecclient.Client, deploymentID string) error {
	if err := g.RunBlocking(ctx); err != nil {
		return fmt.Errorf("cannot run generator: %w", err)
	}

	g.Logger.Info("restarting integrations server to flush apm server data")
	if err := c.RestartIntegrationServer(ctx, deploymentID); err != nil {
		return fmt.Errorf("cannot restart integrations server: %w", err)
	}

	return nil
}
