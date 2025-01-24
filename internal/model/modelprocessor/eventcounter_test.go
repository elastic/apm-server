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

package modelprocessor_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/internal/model/modelprocessor"
)

func TestEventCounter(t *testing.T) {
	batch := modelpb.Batch{
		{},
		{Transaction: &modelpb.Transaction{Type: "transaction_type"}},
		{Span: &modelpb.Span{Type: "span_type"}},
		{Transaction: &modelpb.Transaction{Type: "transaction_type"}},
	}

	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	processor := modelprocessor.NewEventCounter(mp)
	err := processor.ProcessBatch(context.Background(), &batch)
	assert.NoError(t, err)

	monitoringtest.ExpectContainOtelMetrics(t, reader, map[string]any{
		"apm-server.processor.span.transformations":        1,
		"apm-server.processor.transaction.transformations": 2,
	})
}
