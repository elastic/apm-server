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
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/pkg/errors"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/decoder"
	"github.com/elastic/apm-server/internal/model"
	v2 "github.com/elastic/apm-server/internal/model/modeldecoder/v2"
	"github.com/elastic/apm-server/internal/processor/otel"
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

func NewHTTPHandlers(processor model.BatchProcessor) (*otlpreceiver.HTTPHandlers, error) {
	// TODO(axw) stop assuming we have only one OTLP HTTP consumer running
	// at any time, and instead aggregate metrics from consumers that are
	// dynamically registered and unregistered.
	consumer := &otel.Consumer{Processor: processor}
	httpMonitoredConsumer.set(consumer)

	tracesHandler, err := otlpreceiver.TracesHTTPHandler(context.Background(), consumer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OTLP trace receiver")
	}
	metricsHandler, err := otlpreceiver.MetricsHTTPHandler(context.Background(), consumer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OTLP metrics receiver")
	}
	defaultLogsHandler, err := otlpreceiver.LogsHTTPHandler(context.Background(), consumer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OTLP logs receiver")
	}
	withMetaLogsHandler, err := otlpreceiver.LogsWithMetadataHTTPHandler(context.Background(), consumer, extractMetaAndOTLPBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OTLP with metadata logs receiver")
	}
	return &otlpreceiver.HTTPHandlers{
		TraceHandler:   tracesHandler,
		MetricsHandler: metricsHandler,
		LogsHandler: func(w http.ResponseWriter, r *http.Request) {
			mType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil && mType == "multipart/form-data" {
				withMetaLogsHandler(w, r)
				return
			}
			defaultLogsHandler(w, r)
		},
	}, nil
}

func extractMetaAndOTLPBody(mreader *multipart.Reader) ([]byte, interface{}, error) {
	var body []byte
	baseEvent := model.APMEvent{Processor: model.LogProcessor}

	for {
		part, err := mreader.NextPart()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, baseEvent, err
		}

		switch part.FormName() {
		case "metadata":
			r := &decoder.LimitedReader{R: part, N: 10 * 1024}
			dec := decoder.NewJSONDecoder(r)

			if err := v2.DecodeMetadata(dec, &baseEvent); err != nil {
				if r.N < 0 {
					return nil, baseEvent, errors.Wrap(err, "metadata too large")
				}
				return nil, baseEvent, errors.Wrap(err, "invalid metadata")
			}
		case "otlp":
			body, err = ioutil.ReadAll(part)
			if err != nil {
				return nil, baseEvent, err
			}
		}
	}
	return body, baseEvent, nil
}
