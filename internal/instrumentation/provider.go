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

package instrumentation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync/atomic"

	"go.elastic.co/apm/module/apmotel/v2"
	"go.elastic.co/apm/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/elastic/apm-server/internal/version"
	"github.com/elastic/beats/v7/libbeat/instrumentation"
	"github.com/elastic/elastic-agent-client/v7/pkg/client"
	"github.com/elastic/elastic-agent-libs/config"
)

type Provider struct {
	tracer   atomic.Value
	cancel   context.CancelFunc
	listener net.Listener
}

func New(opts ...Option) (*Provider, error) {
	cfg := &cfg{}
	for _, opt := range opts {
		opt(cfg)
	}

	p := &Provider{}

	if !cfg.isManaged && false {
		if err := p.updateTracer(cfg.baseCfg); err != nil {
			return nil, fmt.Errorf("failed to update tracer: %w", err)
		}

		return p, nil
	}

	ver := client.VersionInfo{
		Name:    "apm-server",
		Version: version.Version,
		//BuildHash: version.CommitHash(),
	}
	c, _, err := client.NewV2FromReader(os.Stdin, ver)
	if err != nil {
		return nil, fmt.Errorf("failed to create new v2 client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	if err := c.Start(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start agent v2 client: %w", err)
	}

	p.cancel = cancel

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case change := <-c.UnitChanges():
				if change.Triggers == client.TriggeredAPMChange {
					apmConfig := change.Unit.Expected().APMConfig.Elastic
					bb, err := protojson.Marshal(apmConfig)
					if err != nil {
						panic(err)
					}
					var m map[string]any
					if err := json.Unmarshal(bb, &m); err != nil {
						panic(err)
					}
					newCfg, err := config.NewConfigFrom(m)
					if err != nil {
						panic(err)
					}

					if err := p.updateTracer(newCfg); err != nil {
						panic(err)
					}
				}
			case err := <-c.Errors():
				if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
					fmt.Fprintf(os.Stderr, "GRPC client error: %+v\n", err)
				}
			}
		}
	}()

	return p, nil
}

func (p *Provider) updateTracer(rawConfig *config.C) error {
	instrumentation, err := instrumentation.New(rawConfig, "apm-server", version.Version)
	if err != nil {
		return fmt.Errorf("failed to create instrumentation: %w", err)
	}

	tracer := instrumentation.Tracer()
	p.listener = instrumentation.Listener()

	tracerProvider, err := apmotel.NewTracerProvider(apmotel.WithAPMTracer(tracer))
	if err != nil {
		return fmt.Errorf("failed to create trace provider: %w", err)
	}
	otel.SetTracerProvider(tracerProvider)

	exporter, err := apmotel.NewGatherer()
	if err != nil {
		return fmt.Errorf("failed to create gatherer: %w", err)
	}
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(exporter),
	)
	otel.SetMeterProvider(meterProvider)
	tracer.RegisterMetricsGatherer(exporter)

	p.tracer.Store(tracer)

	return nil
}

func (p *Provider) Tracer() *apm.Tracer {
	return p.tracer.Load().(*apm.Tracer)
}

func (p *Provider) Listener() net.Listener {
	return p.listener
}

func (p *Provider) Close() error {
	p.Tracer().Close()
	p.cancel()
	return nil
}
