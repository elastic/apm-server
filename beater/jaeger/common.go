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

package jaeger

import (
	"context"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/jaegertracing/jaeger/model"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	trjaeger "github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"
)

const (
	collectorType = "jaeger"
)

var (
	monitoringKeys = []request.ResultID{
		request.IDRequestCount, request.IDResponseCount, request.IDResponseErrorsCount, request.IDResponseValidCount,
		request.IDEventReceivedCount, request.IDEventDroppedCount,
	}
)

type monitoringMap map[request.ResultID]*monitoring.Int

func (m monitoringMap) inc(id request.ResultID) {
	if counter, ok := m[id]; ok {
		counter.Inc()
	}
}

func (m monitoringMap) add(id request.ResultID, n int64) {
	if counter, ok := m[id]; ok {
		counter.Add(n)
	}
}

func consumeBatch(
	ctx context.Context,
	batch model.Batch,
	consumer consumer.TraceConsumer,
	requestMetrics monitoringMap,
) error {
	spanCount := int64(len(batch.Spans))
	requestMetrics.add(request.IDEventReceivedCount, spanCount)
	traceData, err := trjaeger.ProtoBatchToOCProto(batch)
	if err != nil {
		requestMetrics.add(request.IDEventDroppedCount, spanCount)
		return err
	}
	traceData.SourceFormat = collectorType
	return consumer.ConsumeTraceData(ctx, traceData)
}
