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

package otlp

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
	"go.uber.org/zap"

	"github.com/elastic/apm-data/model/modelpb"
)

// ConsumeLogsResult contains the number of rejected log records and error message for partial success response.
type ConsumeLogsResult struct {
	ErrorMessage       string
	RejectedLogRecords int64
}

// ConsumeLogs calls ConsumeLogsWithResult but ignores the result.
// It exists to satisfy the go.opentelemetry.io/collector/consumer.Logs interface.
func (c *Consumer) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	_, err := c.ConsumeLogsWithResult(ctx, logs)
	return err
}

// ConsumeLogsWithResult consumes OpenTelemetry log data, converting into
// the Elastic APM log model and sending to the reporter.
func (c *Consumer) ConsumeLogsWithResult(ctx context.Context, logs plog.Logs) (ConsumeLogsResult, error) {
	if err := semAcquire(ctx, c.sem, 1); err != nil {
		return ConsumeLogsResult{}, err
	}
	defer c.sem.Release(1)

	receiveTimestamp := time.Now()
	c.config.Logger.Debug("consuming logs", zap.Stringer("logs", logsStringer(logs)))
	resourceLogs := logs.ResourceLogs()
	batch := make(modelpb.Batch, 0, resourceLogs.Len())
	for i := 0; i < resourceLogs.Len(); i++ {
		c.convertResourceLogs(resourceLogs.At(i), receiveTimestamp, &batch)
	}
	if err := c.config.Processor.ProcessBatch(ctx, &batch); err != nil {
		return ConsumeLogsResult{}, err
	}
	return ConsumeLogsResult{RejectedLogRecords: 0}, nil
}

func (c *Consumer) convertResourceLogs(resourceLogs plog.ResourceLogs, receiveTimestamp time.Time, out *modelpb.Batch) {
	var timeDelta time.Duration
	resource := resourceLogs.Resource()
	baseEvent := modelpb.APMEvent{}
	baseEvent.Event = &modelpb.Event{}
	baseEvent.Event.Received = modelpb.FromTime(receiveTimestamp)
	translateResourceMetadata(resource, &baseEvent)

	if exportTimestamp, ok := exportTimestamp(resource); ok {
		timeDelta = receiveTimestamp.Sub(exportTimestamp)
	}
	scopeLogs := resourceLogs.ScopeLogs()
	for i := 0; i < scopeLogs.Len(); i++ {
		c.convertInstrumentationLibraryLogs(scopeLogs.At(i), &baseEvent, timeDelta, out)
	}
}

func (c *Consumer) convertInstrumentationLibraryLogs(
	in plog.ScopeLogs,
	baseEvent *modelpb.APMEvent,
	timeDelta time.Duration,
	out *modelpb.Batch,
) {
	otelLogs := in.LogRecords()
	for i := 0; i < otelLogs.Len(); i++ {
		event := c.convertLogRecord(otelLogs.At(i), in.Scope(), baseEvent, timeDelta)
		*out = append(*out, event)
	}
}

func (c *Consumer) convertLogRecord(
	record plog.LogRecord,
	scope pcommon.InstrumentationScope,
	baseEvent *modelpb.APMEvent,
	timeDelta time.Duration,
) *modelpb.APMEvent {
	event := baseEvent.CloneVT()
	initEventLabels(event)

	translateScopeMetadata(scope, event)

	if record.Timestamp() == 0 {
		event.Timestamp = modelpb.FromTime(record.ObservedTimestamp().AsTime().Add(timeDelta))
	} else {
		event.Timestamp = modelpb.FromTime(record.Timestamp().AsTime().Add(timeDelta))
	}
	if event.Event == nil {
		event.Event = &modelpb.Event{}
	}
	event.Event.Severity = uint64(record.SeverityNumber())
	if event.Log == nil {
		event.Log = &modelpb.Log{}
	}
	event.Log.Level = record.SeverityText()
	if body := record.Body(); body.Type() != pcommon.ValueTypeEmpty {
		event.Message = body.AsString()
		if body.Type() == pcommon.ValueTypeMap {
			setLabels(body.Map(), event)
		}
	}
	if traceID := record.TraceID(); !traceID.IsEmpty() {
		event.Trace = &modelpb.Trace{}
		event.Trace.Id = hex.EncodeToString(traceID[:])
	}
	if spanID := record.SpanID(); !spanID.IsEmpty() {
		if event.Span == nil {
			event.Span = &modelpb.Span{}
		}
		event.Span.Id = hex.EncodeToString(spanID[:])
	}
	attrs := record.Attributes()

	var exceptionMessage string
	var exceptionStacktrace string
	var exceptionType string
	var exceptionEscaped bool
	var eventName string
	var eventDomain string
	attrs.Range(func(k string, v pcommon.Value) bool {
		switch k {
		case semconv.AttributeExceptionMessage:
			exceptionMessage = v.Str()
		case semconv.AttributeExceptionStacktrace:
			exceptionStacktrace = v.Str()
		case semconv.AttributeExceptionType:
			exceptionType = v.Str()
		case semconv.AttributeExceptionEscaped:
			exceptionEscaped = v.Bool()
		case "event.name":
			eventName = v.Str()
		case "event.domain":
			eventDomain = v.Str()
		case "session.id":
			if event.Session == nil {
				event.Session = &modelpb.Session{}
			}
			event.Session.Id = v.Str()
		case attributeNetworkConnectionType:
			if event.Network == nil {
				event.Network = &modelpb.Network{}
			}
			if event.Network.Connection == nil {
				event.Network.Connection = &modelpb.NetworkConnection{}
			}
			event.Network.Connection.Type = v.Str()
		// data_stream.*
		case attributeDataStreamDataset:
			if event.DataStream == nil {
				event.DataStream = &modelpb.DataStream{}
			}
			event.DataStream.Dataset = v.Str()
		case attributeDataStreamNamespace:
			if event.DataStream == nil {
				event.DataStream = &modelpb.DataStream{}
			}
			event.DataStream.Namespace = v.Str()
		default:
			setLabel(replaceDots(k), event, ifaceAttributeValue(v))
		}
		return true
	})

	// NOTE: we consider an error anything that contains an exception type
	// or message, indipendently from the severity level.
	if exceptionMessage != "" || exceptionType != "" {
		event.Error = convertOpenTelemetryExceptionSpanEvent(
			exceptionType, exceptionMessage, exceptionStacktrace,
			exceptionEscaped, event.Service.Language.Name,
		)
	}

	// We need to check if the "event.name" has the "device" prefix based on the removal of the "event.domain" attribute
	// done in the OTel semantic conventions version 1.24.0.
	if (eventDomain == "device" && eventName != "") || strings.HasPrefix(eventName, "device.") {
		event.Event.Category = "device"
		action := strings.TrimPrefix(eventName, "device.")
		if action == "crash" {
			if event.Error == nil {
				event.Error = &modelpb.Error{}
			}
			event.Error.Type = "crash"
		} else {
			event.Event.Kind = "event"
			event.Event.Action = action
		}
	}

	if event.Error != nil {
		event.Event.Kind = "event"
		event.Event.Type = "error"
	}

	return event
}

func setLabels(m pcommon.Map, event *modelpb.APMEvent) {
	m.Range(func(k string, v pcommon.Value) bool {
		setLabel(replaceDots(k), event, ifaceAttributeValue(v))
		return true
	})
}
