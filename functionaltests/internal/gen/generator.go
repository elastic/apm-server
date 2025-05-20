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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/elastic/apm-perf/pkg/supportedstacks"
	"github.com/elastic/apm-perf/pkg/telemetrygen"

	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
)

type Generator struct {
	logger       *zap.Logger
	kbc          *kbclient.Client
	apmAPIKey    string
	apmServerURL string
}

func New(url, apikey string, kbc *kbclient.Client, logger *zap.Logger) *Generator {
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

func (g *Generator) waitForAPMToBePublishReady(ctx context.Context, maxWaitDuration time.Duration) error {
	timer := time.NewTimer(maxWaitDuration)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return fmt.Errorf("apm server not yet ready after %s", maxWaitDuration)
		default:
			info, err := queryAPMInfo(ctx, g.apmServerURL, g.apmAPIKey)
			if err != nil {
				// Log error and continue in loop.
				g.logger.Warn("failed to query apm info", zap.Error(err))
			} else if info.PublishReady {
				// Ready to publish events to APM.
				return nil
			}

			time.Sleep(1 * time.Second)
		}
	}
}

// runBlocking runs the underlying generator in blocking mode.
func (g *Generator) runBlocking(ctx context.Context, version ecclient.StackVersion) error {
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

	g.logger.Info("wait for apm server to be ready")
	if err = g.waitForAPMToBePublishReady(ctx, 30*time.Second); err != nil {
		return err
	}

	g.logger.Info("ingest data")
	gen.Logger = g.logger
	return gen.RunBlocking(ctx)
}

// RunBlockingWait runs the underlying generator in blocking mode and waits for all in-flight
// data to be flushed before proceeding. This allows the caller to ensure than 1m aggregation
// metrics are ingested immediately after raw data ingestion, without variable delays.
// This may lead to data loss if the final flush takes more than 30s, which may happen if the
// quantity of data ingested with runBlocking gets too big. The current quantity does not
// trigger this behavior.
func (g *Generator) RunBlockingWait(ctx context.Context, version ecclient.StackVersion, integrations bool) error {
	if err := g.runBlocking(ctx, version); err != nil {
		return fmt.Errorf("cannot run generator: %w", err)
	}

	// With Fleet managed APM server, we can trigger metrics flush.
	if integrations {
		if err := flushAPMMetrics(ctx, g.kbc, version); err != nil {
			return fmt.Errorf("cannot flush apm metrics: %w", err)
		}
		return nil
	}

	// With standalone, we don't have Fleet, so simply just wait for some arbitrary time.
	time.Sleep(90 * time.Second)
	return nil
}

// flushAPMMetrics sends an update to the Fleet APM package policy in order
// to trigger the flushing of in-flight APM metrics.
func flushAPMMetrics(ctx context.Context, kbc *kbclient.Client, version ecclient.StackVersion) error {
	policyID := "elastic-cloud-apm"
	description := fmt.Sprintf("Functional tests %s", version)

	// Sending an update with modifying the description is enough to trigger
	// final aggregations in APM Server and flush of in-flight metrics.
	if err := kbc.UpdatePackagePolicyDescriptionByID(ctx, policyID, version, description); err != nil {
		return fmt.Errorf(
			"cannot update %s package policy description to flush aggregation metrics: %w",
			policyID, err,
		)
	}

	// APM Server needs some time to flush all metrics, and we don't have any
	// visibility on when this completes.
	// NOTE: This value comes from empirical observations.
	time.Sleep(40 * time.Second)
	return nil
}

type apmInfoResp struct {
	Version      string `json:"version"`
	PublishReady bool   `json:"publish_ready"`
}

func queryAPMInfo(ctx context.Context, apmServerURL string, apmAPIKey string) (apmInfoResp, error) {
	var httpClient http.Client
	var empty apmInfoResp

	req, err := http.NewRequest(http.MethodGet, apmServerURL, nil)
	if err != nil {
		return empty, fmt.Errorf("cannot create http request: %w", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("ApiKey %s", apmAPIKey))
	req = req.WithContext(ctx)
	resp, err := httpClient.Do(req)
	if err != nil {
		return empty, fmt.Errorf("cannot send http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
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
