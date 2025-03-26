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
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/elastic/apm-perf/pkg/telemetrygen"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
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
		return fmt.Errorf("cannot parse apm server url: %w", err)
	}
	cfg.ServerURL = u

	if err = cfg.EventRate.Set(g.EventRate); err != nil {
		return fmt.Errorf("cannot set event rate: %w", err)
	}

	gen, err := telemetrygen.New(cfg)
	if err != nil {
		return fmt.Errorf("cannot create telemetrygen generator: %w", err)
	}

	g.Logger.Info("ingest data")
	gen.Logger = g.Logger
	return gen.RunBlocking(ctx)
}

// RunBlockingWait runs the underlying generator in blocking mode and waits for all in-flight
// data to be flushed before proceeding. This allows the caller to ensure than 1m aggregation
// metrics are ingested immediately after raw data ingestion, without variable delays.
// This may lead to data loss if the final flush takes more than 30s, which may happen if the
// quantity of data ingested with RunBlocking gets too big. The current quantity does not
// trigger this behavior.
func (g *Generator) RunBlockingWait(ctx context.Context, kbc *kbclient.Client, version string) error {
	if err := g.RunBlocking(ctx); err != nil {
		return fmt.Errorf("cannot run generator: %w", err)
	}

	if err := flushAPMMetrics(ctx, kbc, version); err != nil {
		return fmt.Errorf("cannot flush apm metrics: %w", err)
	}

	return nil
}

// flushAPMMetrics sends an update to the Fleet APM package policy in order
// to trigger the flushing of in-flight APM metrics.
func flushAPMMetrics(ctx context.Context, kbc *kbclient.Client, version string) error {
	policyID := "elastic-cloud-apm"
	policy, err := kbc.GetPackagePolicyByID(ctx, policyID)
	if err != nil {
		return fmt.Errorf("cannot get elastic-cloud-apm package policy: %w", err)
	}

	// If the package policy version returned from API does not match with
	// expected version, set it ourselves to hopefully circumvent it.
	// Relevant issue: https://github.com/elastic/kibana/issues/215437.
	if !strings.HasPrefix(policy.Package.Version, version) {
		// Set the expected version for this ingestion.
		policy.Package.Version = version
	}
	// Sending an update with modifying the description is enough to trigger
	// final aggregations in APM Server and flush of in-flight metrics.
	policy.Description = fmt.Sprintf("Functional tests %s", version)
	if err = kbc.UpdatePackagePolicyByID(ctx, policyID, kbclient.UpdatePackagePolicyRequest{
		PackagePolicy: policy,
		Force:         false,
	}); err != nil {
		return fmt.Errorf("cannot update elastic-cloud-apm package policy: %w", err)
	}

	// APM Server needs some time to flush all metrics, and we don't have any
	// visibility on when this completes.
	// NOTE: This value comes from empirical observations.
	time.Sleep(10 * time.Second)
	return nil
}
