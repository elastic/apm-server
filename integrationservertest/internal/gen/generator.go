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
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/elastic/apm-perf/pkg/supportedstacks"
	"github.com/elastic/apm-perf/pkg/telemetrygen"

	"github.com/elastic/apm-server/integrationservertest/internal/ech"
	"github.com/elastic/apm-server/integrationservertest/internal/kibana"
)

type Generator struct {
	logger       *zap.Logger
	kbc          *kibana.Client
	apmAPIKey    string
	apmServerURL string
}

func New(url, apikey string, kbc *kibana.Client, logger *zap.Logger) *Generator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Generator{
		logger:       logger,
		kbc:          kbc,
		apmAPIKey:    apikey,
		apmServerURL: url,
	}
}

// RunBlockingWait runs the underlying generator in blocking mode and waits for all in-flight
// data to be flushed before proceeding. This allows the caller to ensure than 1m aggregation
// metrics are ingested immediately after raw data ingestion, without variable delays.
// This may lead to data loss if the final flush takes more than 30s, which may happen if the
// quantity of data ingested with runBlocking gets too big. The current quantity does not
// trigger this behavior.
func (g *Generator) RunBlockingWait(ctx context.Context, version ech.Version, integrations bool) error {
	g.logger.Info("wait for apm server to be ready")
	if err := g.waitForAPMToBePublishReady(ctx); err != nil {
		return fmt.Errorf("failed to wait for apm server: %w", err)
	}

	g.logger.Info("ingest data")
	if err := g.retryRunBlocking(ctx, version, 2); err != nil {
		return fmt.Errorf("cannot run generator: %w", err)
	}

	if integrations {
		g.logger.Info("re-apply apm policy")
		if err := g.reapplyAPMPolicy(ctx, version); err != nil {
			return fmt.Errorf("failed to re-apply apm policy: %w", err)
		}
	}

	// Simply wait for some arbitrary time, for the data to be flushed.
	time.Sleep(200 * time.Second)
	return nil
}

// waitForAPMToBePublishReady waits for APM server to be publish-ready by querying the server.
func (g *Generator) waitForAPMToBePublishReady(ctx context.Context) error {
	maxWaitDuration := 60 * time.Second
	timer := time.NewTimer(maxWaitDuration)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.New("apm server not ready but context done")
		case <-timer.C:
			return fmt.Errorf("apm server not ready after %s", maxWaitDuration)
		default:
			info, err := queryAPMInfo(ctx, g.apmServerURL, g.apmAPIKey)
			if err != nil {
				// Log error and continue in loop.
				g.logger.Warn("failed to query apm info", zap.Error(err))
			} else if info.PublishReady {
				// Ready to publish events to APM.
				return nil
			}

			time.Sleep(10 * time.Second)
		}
	}
}

// runBlocking runs the underlying generator in blocking mode.
func (g *Generator) runBlocking(ctx context.Context, version ech.Version) error {
	eventRate := "1000/s"
	cfg := telemetrygen.DefaultConfig()
	cfg.APIKey = g.apmAPIKey
	cfg.TargetStackVersion = supportedstacks.TargetStackVersionLatest
	if version.Major == 7 {
		eventRate = "200/s" // Using 1000/s resulted in consistent 503 for 7x.
		cfg.TargetStackVersion = supportedstacks.TargetStackVersion7x
	}

	u, err := url.Parse(g.apmServerURL)
	if err != nil {
		return fmt.Errorf("cannot parse apm server url: %w", err)
	}
	cfg.ServerURL = u

	if err = cfg.EventRate.Set(eventRate); err != nil {
		return fmt.Errorf("cannot set event rate: %w", err)
	}

	gen, err := telemetrygen.New(cfg)
	if err != nil {
		return fmt.Errorf("cannot create telemetrygen generator: %w", err)
	}

	gen.Logger = g.logger
	return gen.RunBlocking(ctx)
}

// retryRunBlocking executes runBlocking. If it fails, it will retry up to retryTimes.
func (g *Generator) retryRunBlocking(ctx context.Context, version ech.Version, retryTimes int) error {
	// No error, don't need to retry.
	if err := g.runBlocking(ctx, version); err == nil {
		return nil
	}

	// Otherwise, retry until success or run out of attempts.
	var finalErr error
	for i := 0; i < retryTimes; i++ {
		// Wait for some time before retrying.
		time.Sleep(time.Duration(i) * 30 * time.Second)

		g.logger.Info(fmt.Sprintf("retrying ingest data attempt %d", i+1))
		err := g.runBlocking(ctx, version)
		// Retry success, simply return.
		if err == nil {
			return nil
		}

		finalErr = err
	}

	return finalErr
}

func (g *Generator) reapplyAPMPolicy(ctx context.Context, version ech.Version) error {
	policyID := "elastic-cloud-apm"
	description := fmt.Sprintf("%s %s", version, rand.Text()[:10])

	if err := g.kbc.UpdatePackagePolicyDescriptionByID(ctx, policyID, version, description); err != nil {
		return fmt.Errorf(
			"cannot update %s package policy description: %w",
			policyID, err,
		)
	}

	return nil
}

type apmInfoResp struct {
	Version      string `json:"version"`
	PublishReady bool   `json:"publish_ready"`
}

func queryAPMInfo(ctx context.Context, apmServerURL string, apmAPIKey string) (apmInfoResp, error) {
	var httpClient http.Client
	var empty apmInfoResp

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apmServerURL, nil)
	if err != nil {
		return empty, fmt.Errorf("cannot create http request: %w", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("ApiKey %s", apmAPIKey))
	resp, err := httpClient.Do(req)
	if err != nil {
		return empty, fmt.Errorf("cannot send http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return empty, fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return empty, fmt.Errorf("cannot read response body: %w", err)
	}

	var apmInfo apmInfoResp
	if err = json.Unmarshal(b, &apmInfo); err != nil {
		return empty, fmt.Errorf("cannot unmarshal response body: %w", err)
	}

	return apmInfo, nil
}
