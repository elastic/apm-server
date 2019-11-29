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
	"time"

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
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/transaction"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

type collector interface {
	start() error
	stop()
	name() string
	endpoint() string
}
type otelCollector struct {
	logger     *logp.Logger
	collectors []collector
	observer   *observer
}

func newOtelCollector(logger *logp.Logger, cfg *config.Config, tracer *apm.Tracer, reporter publish.Reporter) (*otelCollector, error) {
	observer := newObserver(context.Background())
	traceConsumer := &Consumer{
		Reporter:        reporter,
		TransformConfig: transform.Config{},
	}
	otelCollector := &otelCollector{logger, []collector{}, observer}

	jaegerCollector, err := newJaegerCollector(cfg, traceConsumer, tracer, observer)
	if err != nil {
		return nil, err
	}
	if jaegerCollector != nil {
		otelCollector.collectors = append(otelCollector.collectors, jaegerCollector)
	}

	return otelCollector, nil
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

type observer struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan error
}

func newObserver(ctx context.Context) *observer {
	ctx, cancel := context.WithCancel(ctx)
	return &observer{ctx: ctx, cancel: cancel, ch: make(chan error)}
}

func (o *observer) fatal(err error) {
	o.ch <- err
}

type jaegerCollector struct {
	grpcServer *grpc.Server
	host       string
	receiver   *jaegerReceiver
	observer   *observer
}

func newJaegerCollector(cfg *config.Config, traceConsumer *Consumer, tracer *apm.Tracer, observer *observer) (*jaegerCollector, error) {
	if cfg.OtelConfig == nil || !cfg.OtelConfig.Jaeger.Enabled {
		return nil, nil
	}
	return &jaegerCollector{
		receiver: &jaegerReceiver{traceConsumer: traceConsumer},
		grpcServer: grpc.NewServer(grpc.UnaryInterceptor(apmgrpc.NewUnaryServerInterceptor(
			apmgrpc.WithRecovery(),
			apmgrpc.WithTracer(tracer)))),
		host:     cfg.OtelConfig.Jaeger.GRPC.Host,
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
			jc.observer.fatal(err)
		}
	}()
	return nil
}

func (jc *jaegerCollector) stop() {
	jc.grpcServer.GracefulStop()
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

// PostSpans implements the api_v2/collector.proto. It converts spans received via Jaeger Proto batch to open-telemetry
// TraceData and passes them on to the internal Consumer taking care of converting into Elastic APM format.
// The implementation of the protobuf contract is based on the open-telemetry implementation at
// https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/jaegerreceiver
func (jr *jaegerReceiver) PostSpans(ctx context.Context, r *api_v2.PostSpansRequest) (*api_v2.PostSpansResponse, error) {
	ctxWithReceiverName := observability.ContextWithReceiverName(ctx, jaegerReceiverName)
	spansCount := len(r.Batch.Spans)
	traceData, err := jaeger.ProtoBatchToOCProto(r.Batch)
	if err != nil {
		observability.RecordMetricsForTraceReceiver(ctxWithReceiverName, spansCount, spansCount)
		return nil, err
	}
	traceData.SourceFormat = jaegerType
	observability.RecordMetricsForTraceReceiver(ctxWithReceiverName, spansCount, spansCount-len(traceData.Spans))

	if err = jr.traceConsumer.ConsumeTraceData(ctx, traceData); err != nil {
		return nil, err
	}
	return &api_v2.PostSpansResponse{}, nil
}

const (
	jaegerType         = "jaeger"
	jaegerReceiverName = jaegerType + "-collector"
	networkTCP         = "tcp"
)

// Consumer is a place holder for proper implementation of transforming open-telemetry to elastic APM conform data
// This needs to be moved to the processor package when implemented properly
//TODO(simi): move to processors when implementing
type Consumer struct {
	Reporter        publish.Reporter
	TransformConfig transform.Config
}

// ConsumeTraceData is only a place holder right now, needs to be implemented.
func (c *Consumer) ConsumeTraceData(ctx context.Context, td consumerdata.TraceData) error {
	transformables := []transform.Transformable{}
	for _, traceData := range td.Spans {
		transformables = append(transformables, &transaction.Event{
			TraceId: fmt.Sprintf("%x", traceData.TraceId),
			Id:      fmt.Sprintf("%x", traceData.SpanId),
		})
	}
	transformContext := &transform.Context{
		RequestTime: time.Now(),
		Config:      c.TransformConfig,
		Metadata:    metadata.Metadata{},
	}
	return c.Reporter(ctx, publish.PendingReq{
		Transformables: transformables,
		Tcontext:       transformContext,
		Trace:          true,
	})
}
