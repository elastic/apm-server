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

package otlp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/elastic/apm-data/input"
	"github.com/elastic/apm-data/input/otlp"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
)

var (
	httpMetricRegistrationMu          sync.Mutex
	unsupportedHTTPMetricRegistration metric.Registration
)

func NewHTTPHandlers(logger *zap.Logger, processor modelpb.BatchProcessor, semaphore input.Semaphore, mp metric.MeterProvider, tp trace.TracerProvider) HTTPHandlers {
	// TODO(axw) stop assuming we have only one OTLP HTTP consumer running
	// at any time, and instead aggregate metrics from consumers that are
	// dynamically registered and unregistered.
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor:        modelprocessor.NewTracer("otlp.ProcessBatch", processor, modelprocessor.WithTracerProvider(tp)),
		Logger:           logger,
		Semaphore:        semaphore,
		RemapOTelMetrics: true,
		TraceProvider:    tp,
	})

	meter := mp.Meter("github.com/elastic/apm-server/internal/beater/otlp")
	httpMetricsConsumerUnsupportedDropped, _ := meter.Int64ObservableCounter(
		"apm-server.otlp.http.metrics.consumer.unsupported_dropped",
	)

	httpMetricRegistrationMu.Lock()
	defer httpMetricRegistrationMu.Unlock()

	// TODO we should add an otel counter metric directly in the
	// apm-data consumer, then we could get rid of the callback.
	if unsupportedHTTPMetricRegistration != nil {
		_ = unsupportedHTTPMetricRegistration.Unregister()
	}
	unsupportedHTTPMetricRegistration, _ = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		stats := consumer.Stats()
		if stats.UnsupportedMetricsDropped > 0 {
			o.ObserveInt64(httpMetricsConsumerUnsupportedDropped, stats.UnsupportedMetricsDropped)
		}
		return nil
	}, httpMetricsConsumerUnsupportedDropped)

	return HTTPHandlers{consumer: consumer}
}

// HTTPHandlers encapsulates http.HandlerFuncs for handling traces, metrics, and logs.
type HTTPHandlers struct {
	consumer *otlp.Consumer
}

// HandleTraces is an http.HandlerFunc that receives a protobuf-encoded traces export
// request, and processes it with the handler's OTLP consumer.
func (h HTTPHandlers) HandleTraces(w http.ResponseWriter, r *http.Request) {
	req := ptraceotlp.NewExportRequest()
	if err := h.readRequest(r, req); err != nil {
		h.writeError(w, err, http.StatusBadRequest)
		return
	}
	var result otlp.ConsumeTracesResult
	var err error
	if result, err = h.consumer.ConsumeTracesWithResult(r.Context(), req.Traces()); err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}
	resp := ptraceotlp.NewExportResponse()
	if result.RejectedSpans > 0 {
		resp.PartialSuccess().SetRejectedSpans(result.RejectedSpans)
		resp.PartialSuccess().SetErrorMessage(result.ErrorMessage)
	}
	if err := h.writeResponse(w, resp); err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}
}

// HandleMetrics is an http.HandlerFunc that receives a protobuf-encoded metrics export
// request, and processes it with the handler's OTLP consumer.
func (h HTTPHandlers) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	req := pmetricotlp.NewExportRequest()
	if err := h.readRequest(r, req); err != nil {
		h.writeError(w, err, http.StatusBadRequest)
		return
	}
	var result otlp.ConsumeMetricsResult
	var err error
	if result, err = h.consumer.ConsumeMetricsWithResult(r.Context(), req.Metrics()); err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}
	resp := pmetricotlp.NewExportResponse()
	if result.RejectedDataPoints > 0 {
		resp.PartialSuccess().SetRejectedDataPoints(result.RejectedDataPoints)
		resp.PartialSuccess().SetErrorMessage(result.ErrorMessage)
	}
	if err := h.writeResponse(w, resp); err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}
}

// HandleLogs is an http.HandlerFunc that receives a protobuf-encoded logs export
// request, and processes it with the handler's OTLP consumer.
func (h HTTPHandlers) HandleLogs(w http.ResponseWriter, r *http.Request) {
	req := plogotlp.NewExportRequest()
	if err := h.readRequest(r, req); err != nil {
		h.writeError(w, err, http.StatusBadRequest)
		return
	}
	var result otlp.ConsumeLogsResult
	var err error
	if result, err = h.consumer.ConsumeLogsWithResult(r.Context(), req.Logs()); err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}
	resp := plogotlp.NewExportResponse()
	if result.RejectedLogRecords > 0 {
		resp.PartialSuccess().SetRejectedLogRecords(result.RejectedLogRecords)
		resp.PartialSuccess().SetErrorMessage(result.ErrorMessage)
	}
	if err := h.writeResponse(w, resp); err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}
}

type protoUnmarshaler interface {
	UnmarshalProto([]byte) error
}

type protoMarshaler interface {
	MarshalProto() ([]byte, error)
}

func (h HTTPHandlers) readRequest(req *http.Request, out protoUnmarshaler) error {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}
	if err := out.UnmarshalProto(body); err != nil {
		return fmt.Errorf("failed to unmarshal request body: %w", err)
	}
	return nil
}

func (h HTTPHandlers) writeResponse(w http.ResponseWriter, m protoMarshaler) error {
	body, err := m.MarshalProto()
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
	return nil
}

func (h HTTPHandlers) writeError(w http.ResponseWriter, err error, statusCode int) {
	s, ok := status.FromError(err)
	if !ok {
		if statusCode == http.StatusBadRequest {
			s = status.New(codes.InvalidArgument, err.Error())
		} else {
			s = status.New(codes.Unknown, err.Error())
		}
	}
	msg, err := proto.Marshal(s.Proto())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"code": 13, "message": "failed to marshal error message"}`))
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(statusCode)
	w.Write(msg)
}
