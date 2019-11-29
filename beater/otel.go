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
	"net"

	"github.com/elastic/beats/libbeat/logp"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/observability"
	"github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"
	"github.com/pkg/errors"
	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmgrpc"
	"google.golang.org/grpc"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

type collector interface {
	start() error
	stop()
	name() string
	endpoint() string
}

type observer struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan error
}

func newObserver(ctx context.Context) *observer {
	ctx, cancel := context.WithCancel(ctx)
	return &observer{ctx: ctx, cancel: cancel, ch: make(chan error)}
}

func (o *observer) Fatal(err error) {
	o.ch <- err
}

type otelCollector struct {
	logger     *logp.Logger
	collectors []collector
	observer   *observer
}

func newOtelCollector(logger *logp.Logger, cfg *config.Config, tracer *apm.Tracer, reporter publish.Reporter) (*otelCollector, error) {
	observer := newObserver(context.Background())
	traceConsumer := &Consumer{
		TransformConfig: transform.Config{},
		ModelConfig:     model.Config{Experimental: cfg.Mode == config.ModeExperimental},
		Reporter:        reporter,
	}

	jaegerCollector, err := newJaegerCollector(traceConsumer, tracer, observer)
	if err != nil {
		return nil, err
	}

	return &otelCollector{logger, []collector{jaegerCollector}, observer}, nil
}

func (otc *otelCollector) start() error {
	for _, c := range otc.collectors {
		otc.logger.Infof("Starting collector %s listening on: %s", c.name(), c.endpoint())
		if err := c.start(); err != nil {
			return errors.Wrapf(err, "error starting collector %s listening on: %s", c.name(), c.endpoint())
		}
	}
	select {
	case <-otc.observer.ctx.Done():
		return otc.observer.ctx.Err()
	case err := <-otc.observer.ch:
		return err
	}
}

func (otc *otelCollector) stop() {
	otc.observer.cancel()
	for _, c := range otc.collectors {
		otc.logger.Infof("Stopping collector %s listening on: %s", c.name(), c.endpoint())
		c.stop()
	}
}

type jaegerCollector struct {
	grpcServer *grpc.Server
	host       string
	receiver   *jaegerReceiver
	observer   *observer
}

func newJaegerCollector(traceConsumer *Consumer, tracer *apm.Tracer, observer *observer) (*jaegerCollector, error) {
	return &jaegerCollector{
		receiver: &jaegerReceiver{traceConsumer: traceConsumer},
		grpcServer: grpc.NewServer(grpc.UnaryInterceptor(apmgrpc.NewUnaryServerInterceptor(
			apmgrpc.WithRecovery(),
			apmgrpc.WithTracer(tracer)))),
		host:     grpcEndpoint,
		observer: observer,
	}, nil
}

func (jc *jaegerCollector) start() error {
	api_v2.RegisterCollectorServiceServer(jc.grpcServer, jc.receiver)
	listener, err := net.Listen(networkTCP, jc.host)
	if err != nil {
		return err
	}
	go func() {
		if err := jc.grpcServer.Serve(listener); err != nil {
			jc.observer.Fatal(err)
		}
	}()
	return nil
}

func (js *jaegerCollector) stop() {
	js.grpcServer.GracefulStop()
}

func (jc *jaegerCollector) name() string {
	return jaegerType
}

func (jc *jaegerCollector) endpoint() string {
	return jc.host
}

type jaegerReceiver struct {
	traceConsumer *Consumer
}

func (jr *jaegerReceiver) PostSpans(ctx context.Context, r *api_v2.PostSpansRequest) (*api_v2.PostSpansResponse, error) {
	ctxWithReceiverName := observability.ContextWithReceiverName(ctx, jaegerReceiverName)
	spansCount := len(r.Batch.Spans)
	traceData, err := jaeger.ProtoBatchToOCProto(r.Batch)
	if err != nil {
		observability.RecordMetricsForTraceReceiver(ctxWithReceiverName, spansCount, spansCount)
		return nil, err
	}
	traceData.SourceFormat = "jaeger"
	observability.RecordMetricsForTraceReceiver(ctxWithReceiverName, spansCount, spansCount-len(traceData.Spans))

	if err = jr.traceConsumer.ConsumeTraceData(ctx, traceData); err != nil {
		return nil, err
	}
	return &api_v2.PostSpansResponse{}, nil
}

const (
	jaegerType         = "jaeger"
	jaegerReceiverName = jaegerType + "-collector"
	grpcEndpoint       = "localhost:14250"
	networkTCP         = "tcp"
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
