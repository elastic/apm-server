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

	"github.com/jaegertracing/jaeger/model"
	"go.opentelemetry.io/collector/consumer"
	trjaeger "go.opentelemetry.io/collector/translator/trace/jaeger"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
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
	consumer consumer.Traces,
	requestMetrics monitoringMap,
) error {
	spanCount := int64(len(batch.Spans))
	requestMetrics.add(request.IDEventReceivedCount, spanCount)
	traces := trjaeger.ProtoBatchToInternalTraces(batch)
	return consumer.ConsumeTraces(ctx, traces)
}
