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

// Portions copied from OpenTelemetry Collector (contrib), from the
// elastic exporter.
//
// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otel_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	semconv "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/otel"
)

func TestConsumerConsumeLogs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var processor model.ProcessBatchFunc = func(_ context.Context, batch *model.Batch) error {
			assert.Empty(t, batch)
			return nil
		}

		consumer := otel.Consumer{Processor: processor}
		logs := pdata.NewLogs()
		assert.NoError(t, consumer.ConsumeLogs(context.Background(), logs))
	})

	commonEvent := model.APMEvent{
		Processor: model.LogProcessor,
		Agent: model.Agent{
			Name:    "otlp/go",
			Version: "unknown",
		},
		Service: model.Service{
			Name:     "unknown",
			Language: model.Language{Name: "go"},
		},
		Message: "a random log message",
		Event: model.Event{
			Severity: int64(pdata.SeverityNumberINFO),
			Action:   "doOperation()",
		},
		Log:   model.Log{Level: "Info"},
		Span:  &model.Span{ID: "0200000000000000"},
		Trace: model.Trace{ID: "01000000000000000000000000000000"},
		Labels: model.Labels{
			"key": model.LabelValue{Value: "value"},
		},
		NumericLabels: model.NumericLabels{
			"numeric_key": model.NumericLabelValue{Value: 1234},
		},
	}
	test := func(name string, body interface{}, expectedMessage string) {
		t.Run(name, func(t *testing.T) {
			logs := newLogs(body)

			var processed model.Batch
			var processor model.ProcessBatchFunc = func(_ context.Context, batch *model.Batch) error {
				if processed != nil {
					panic("already processes batch")
				}
				processed = *batch
				assert.NotNil(t, processed[0].Timestamp)
				processed[0].Timestamp = time.Time{}
				return nil
			}
			consumer := otel.Consumer{Processor: processor}
			assert.NoError(t, consumer.ConsumeLogs(context.Background(), logs))

			expected := commonEvent
			expected.Message = expectedMessage
			assert.Equal(t, model.Batch{expected}, processed)
		})
	}
	test("string_body", "a random log message", "a random log message")
	test("int_body", 1234, "1234")
	test("float_body", 1234.1234, "1234.1234")
	test("bool_body", true, "true")
	// TODO(marclop): How to test map body
}

func newLogs(body interface{}) pdata.Logs {
	logs := pdata.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	logs.ResourceLogs().At(0).Resource().Attributes().InitFromMap(map[string]pdata.AttributeValue{
		semconv.AttributeTelemetrySDKLanguage: pdata.NewAttributeValueString("go"),
	})
	instrumentationLogs := resourceLogs.InstrumentationLibraryLogs().AppendEmpty()
	otelLog := instrumentationLogs.Logs().AppendEmpty()
	otelLog.SetTraceID(pdata.NewTraceID([16]byte{1}))
	otelLog.SetSpanID(pdata.NewSpanID([8]byte{2}))
	otelLog.SetName("doOperation()")
	otelLog.SetSeverityNumber(pdata.SeverityNumberINFO)
	otelLog.SetSeverityText("Info")
	otelLog.SetTimestamp(pdata.NewTimestampFromTime(time.Now()))
	otelLog.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"key":         pdata.NewAttributeValueString("value"),
		"numeric_key": pdata.NewAttributeValueDouble(1234),
	})

	switch b := body.(type) {
	case string:
		otelLog.Body().SetStringVal(b)
	case int:
		otelLog.Body().SetIntVal(int64(b))
	case float64:
		otelLog.Body().SetDoubleVal(b)
	case bool:
		otelLog.Body().SetBoolVal(b)
		// case map[string]string:
		// TODO(marclop) figure out how to set the body since it cannot be set
		// as a map.
		// otelLog.Body()
	}
	return logs
}
