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
	"fmt"

	"github.com/elastic/beats/libbeat/logp"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/receiver"
	"github.com/open-telemetry/opentelemetry-collector/receiver/jaegerreceiver"
	"github.com/pkg/errors"
	"go.elastic.co/apm"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

type otcollector struct {
	logger         *logp.Logger
	traceReceivers []receiver.TraceReceiver
	host           *host
}

func newOTCollectors(logger *logp.Logger, cfg *config.Config, _ *apm.Tracer, reporter publish.Reporter) (*otcollector, error) {
	traceConsumer := &Consumer{
		TransformConfig: transform.Config{},
		ModelConfig:     model.Config{Experimental: cfg.Mode == config.ModeExperimental},
		Reporter:        reporter,
	}

	ctx := context.Background()
	host := newHost(ctx)

	factory := &jaegerreceiver.Factory{}
	//TODO(simi): make port configurable
	receiverCfg := &jaegerreceiver.Config{
		TypeVal: jaegerType,
		NameVal: jaegerType,
		Protocols: map[string]*receiver.SecureReceiverSettings{
			protoGRPC: {
				ReceiverSettings: configmodels.ReceiverSettings{
					Endpoint: defaultGRPCBindEndpoint,
				},
			},
		},
	}
	jaegerReceiver, err := factory.CreateTraceReceiver(ctx, nil, receiverCfg, traceConsumer)
	if err != nil {
		return &otcollector{}, errors.Wrapf(err, "error building trace receiver for Jaeger")
	}
	return &otcollector{logger, []receiver.TraceReceiver{jaegerReceiver}, host}, nil
}

func (otc *otcollector) start() error {
	for _, r := range otc.traceReceivers {
		//TODO(simi): remove patch from inside jaegers `jaegerReceiver.StartTraceReception` once
		// https://github.com/open-telemetry/opentelemetry-collector/pull/434 has landed
		otc.logger.Infof("Starting trace receiver for %s", r.TraceSource())
		if err := r.StartTraceReception(otc.host); err != nil {
			return errors.Wrapf(err, "error starting trace receiver for %s", r.TraceSource())
		}
	}
	select {
	case <-otc.host.ctx.Done():
		return otc.host.ctx.Err()
	case err := <-otc.host.ch:
		return err
	}
}

func (otc *otcollector) stop() {
	otc.host.cancel()
	for _, r := range otc.traceReceivers {
		otc.logger.Infof("Stopping trace receiver for %s", r.TraceSource())
		if err := r.StopTraceReception(); err != nil {
			otc.logger.Errorf("error stopping trace receiver for %s: %s", r.TraceSource(), err.Error())
		}
	}
}

type host struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan error
}

func newHost(ctx context.Context) *host {
	ctx, cancel := context.WithCancel(ctx)
	return &host{ctx: ctx, cancel: cancel, ch: make(chan error)}
}

// Context returns a context provided by the host to be used on the receiver
// operations.
func (h *host) Context() context.Context {
	return h.ctx
}

// ReportFatalError is used to report to the host that the receiver encountered
// a fatal error (i.e.: an error that the instance can't recover from) after
// its start function has already returned.
func (h *host) ReportFatalError(err error) {
	h.ch <- err
}

const (
	jaegerType              = "jaeger"
	protoGRPC               = "grpc"
	defaultGRPCBindEndpoint = "localhost:14250"
)

//TODO(simi): move to processors when implementing
type Consumer struct {
	TransformConfig transform.Config
	ModelConfig     model.Config
	Reporter        publish.Reporter
}

func (c *Consumer) ConsumeTraceData(ctx context.Context, td consumerdata.TraceData) error {
	fmt.Println("------------------------------------ CONSUMING trace data")
	return nil
}
