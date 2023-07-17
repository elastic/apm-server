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
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/elastic/apm-data/input"
	"github.com/elastic/apm-data/input/otlp"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

var (
	httpMetricsRegistry      = monitoring.Default.NewRegistry("apm-server.otlp.http.metrics")
	HTTPMetricsMonitoringMap = request.MonitoringMapForRegistry(httpMetricsRegistry, monitoringKeys)
	httpTracesRegistry       = monitoring.Default.NewRegistry("apm-server.otlp.http.traces")
	HTTPTracesMonitoringMap  = request.MonitoringMapForRegistry(httpTracesRegistry, monitoringKeys)
	httpLogsRegistry         = monitoring.Default.NewRegistry("apm-server.otlp.http.logs")
	HTTPLogsMonitoringMap    = request.MonitoringMapForRegistry(httpLogsRegistry, monitoringKeys)

	httpMonitoredConsumer monitoredConsumer
)

func init() {
	monitoring.NewFunc(httpMetricsRegistry, "consumer", httpMonitoredConsumer.collect, monitoring.Report)
}

func NewHTTPHandlers(logger *zap.Logger, processor modelpb.BatchProcessor, semaphore input.Semaphore) HTTPHandlers {
	// TODO(axw) stop assuming we have only one OTLP HTTP consumer running
	// at any time, and instead aggregate metrics from consumers that are
	// dynamically registered and unregistered.
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: processor,
		Logger:    logger,
		Semaphore: semaphore,
	})
	httpMonitoredConsumer.set(consumer)
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
	if err := h.consumer.ConsumeTraces(r.Context(), req.Traces()); err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}
	if err := h.writeResponse(w, ptraceotlp.NewExportResponse()); err != nil {
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
	if err := h.consumer.ConsumeMetrics(r.Context(), req.Metrics()); err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}
	if err := h.writeResponse(w, pmetricotlp.NewExportResponse()); err != nil {
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
	if err := h.consumer.ConsumeLogs(r.Context(), req.Logs()); err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}
	if err := h.writeResponse(w, plogotlp.NewExportResponse()); err != nil {
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
