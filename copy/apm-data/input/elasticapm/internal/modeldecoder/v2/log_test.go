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

package v2

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-data/input/elasticapm/internal/decoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder"
	"github.com/elastic/apm-data/model/modelpb"
)

func TestResetLogOnRelease(t *testing.T) {
	input := `{"log":{"message":"something happened"}}`
	root := fetchLogRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(input)).Decode(root))
	require.True(t, root.IsSet())
	releaseLogRoot(root)
	assert.False(t, root.IsSet())
}

func TestDecodeNestedLog(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		t.Run("withTimestamp", func(t *testing.T) {
			input := modeldecoder.Input{Base: &modelpb.APMEvent{}}
			str := `{"log":{"message":"something happened","@timestamp":1662616971000000,"trace.id":"trace-id","transaction.id":"transaction-id","log.level":"warn","log.logger":"testLogger","log.origin.file.name":"testFile","log.origin.file.line":10,"log.origin.function":"testFunc","service.name":"testsvc","service.version":"v1.2.0","service.environment":"prod","service.node.name":"testNode","process.thread.name":"testThread","event.dataset":"accesslog","labels":{"k":"v"}}}`
			dec := decoder.NewJSONDecoder(strings.NewReader(str))
			var batch modelpb.Batch
			require.NoError(t, DecodeNestedLog(dec, &input, &batch))
			require.Len(t, batch, 1)
			assert.Equal(t, "something happened", batch[0].Message)
			assert.Equal(t, "2022-09-08 06:02:51 +0000 UTC", modelpb.ToTime(batch[0].Timestamp).String())
			assert.Equal(t, "trace-id", batch[0].Trace.Id)
			assert.Equal(t, "transaction-id", batch[0].Transaction.Id)
			assert.Equal(t, "warn", batch[0].Log.Level)
			assert.Equal(t, "testLogger", batch[0].Log.Logger)
			assert.Equal(t, "testFile", batch[0].Log.Origin.File.Name)
			assert.Equal(t, uint32(10), batch[0].Log.Origin.File.Line)
			assert.Equal(t, "testFunc", batch[0].Log.Origin.FunctionName)
			assert.Equal(t, "testsvc", batch[0].Service.Name)
			assert.Equal(t, "v1.2.0", batch[0].Service.Version)
			assert.Equal(t, "prod", batch[0].Service.Environment)
			assert.Equal(t, "testNode", batch[0].Service.Node.Name)
			assert.Equal(t, "testThread", batch[0].Process.Thread.Name)
			assert.Equal(t, "accesslog", batch[0].Event.Dataset)
			assert.Equal(t, modelpb.Labels{"k": &modelpb.LabelValue{Value: "v"}}, modelpb.Labels(batch[0].Labels))
		})

		t.Run("withoutTimestamp", func(t *testing.T) {
			now := modelpb.FromTime(time.Now())
			input := modeldecoder.Input{Base: &modelpb.APMEvent{Timestamp: now}}
			str := `{"log":{"message":"something happened"}}`
			dec := decoder.NewJSONDecoder(strings.NewReader(str))
			var batch modelpb.Batch
			require.NoError(t, DecodeNestedLog(dec, &input, &batch))
			assert.Equal(t, now, batch[0].Timestamp)
		})

		t.Run("withError", func(t *testing.T) {
			input := modeldecoder.Input{Base: &modelpb.APMEvent{}}
			str := `{"log":{"@timestamp":1662616971000000,"trace.id":"trace-id","transaction.id":"transaction-id","log.level":"error","log.logger":"testLogger","log.origin.file.name":"testFile","log.origin.file.line":10,"log.origin.function":"testFunc","service.name":"testsvc","service.version":"v1.2.0","service.environment":"prod","service.node.name":"testNode","process.thread.name":"testThread","event.dataset":"accesslog","labels":{"k":"v"}, "error.type": "illegal-argument", "error.message": "illegal argument received", "error.stack_trace": "stack_trace_as_string"}}`
			dec := decoder.NewJSONDecoder(strings.NewReader(str))
			var batch modelpb.Batch
			require.NoError(t, DecodeNestedLog(dec, &input, &batch))
			require.Len(t, batch, 1)
			assert.Equal(t, "2022-09-08 06:02:51 +0000 UTC", modelpb.ToTime(batch[0].Timestamp).String())
			assert.Equal(t, "trace-id", batch[0].Trace.Id)
			assert.Equal(t, "transaction-id", batch[0].Transaction.Id)
			assert.Equal(t, "error", batch[0].Log.Level)
			assert.Equal(t, "testLogger", batch[0].Log.Logger)
			assert.Equal(t, "testFile", batch[0].Log.Origin.File.Name)
			assert.Equal(t, uint32(10), batch[0].Log.Origin.File.Line)
			assert.Equal(t, "testFunc", batch[0].Log.Origin.FunctionName)
			assert.Equal(t, "testsvc", batch[0].Service.Name)
			assert.Equal(t, "v1.2.0", batch[0].Service.Version)
			assert.Equal(t, "prod", batch[0].Service.Environment)
			assert.Equal(t, "testNode", batch[0].Service.Node.Name)
			assert.Equal(t, "testThread", batch[0].Process.Thread.Name)
			assert.Equal(t, "accesslog", batch[0].Event.Dataset)
			assert.Equal(t, "illegal-argument", batch[0].Error.Type)
			assert.Equal(t, "illegal argument received", batch[0].Error.Message)
			assert.Equal(t, "stack_trace_as_string", batch[0].Error.StackTrace)
			assert.Equal(t, modelpb.Labels{"k": &modelpb.LabelValue{Value: "v"}}, modelpb.Labels(batch[0].Labels))
		})

		t.Run("withNestedJSON", func(t *testing.T) {
			input := modeldecoder.Input{Base: &modelpb.APMEvent{}}
			str := `{"log":{"@timestamp":1662616971000000,"trace.id":"trace-id","transaction.id":"transaction-id","log": {"logger": "testLogger","origin": {"file": {"name": "testFile","line":10},"function": "testFunc"}},"log.level":"error","service": {"name": "testsvc","version": "v1.2.0","environment": "prod","node": {"name": "testNode"}},"process": {"thread": {"name": "testThread"}},"event": {"dataset":"accesslog"},"labels":{"k":"v"},"error": {"type": "illegal-argument","message": "illegal argument received","stack_trace": "stack_trace_as_string"}}}`
			dec := decoder.NewJSONDecoder(strings.NewReader(str))
			var batch modelpb.Batch
			require.NoError(t, DecodeNestedLog(dec, &input, &batch))
			require.Len(t, batch, 1)
			assert.Equal(t, "2022-09-08 06:02:51 +0000 UTC", modelpb.ToTime(batch[0].Timestamp).String())
			assert.Equal(t, "trace-id", batch[0].Trace.Id)
			assert.Equal(t, "transaction-id", batch[0].Transaction.Id)
			assert.Equal(t, "error", batch[0].Log.Level)
			assert.Equal(t, "testLogger", batch[0].Log.Logger)
			assert.Equal(t, "testFile", batch[0].Log.Origin.File.Name)
			assert.Equal(t, uint32(10), batch[0].Log.Origin.File.Line)
			assert.Equal(t, "testFunc", batch[0].Log.Origin.FunctionName)
			assert.Equal(t, "testsvc", batch[0].Service.Name)
			assert.Equal(t, "v1.2.0", batch[0].Service.Version)
			assert.Equal(t, "prod", batch[0].Service.Environment)
			assert.Equal(t, "testNode", batch[0].Service.Node.Name)
			assert.Equal(t, "testThread", batch[0].Process.Thread.Name)
			assert.Equal(t, "accesslog", batch[0].Event.Dataset)
			assert.Equal(t, "illegal-argument", batch[0].Error.Type)
			assert.Equal(t, "illegal argument received", batch[0].Error.Message)
			assert.Equal(t, "stack_trace_as_string", batch[0].Error.StackTrace)
			assert.Equal(t, modelpb.Labels{"k": &modelpb.LabelValue{Value: "v"}}, modelpb.Labels(batch[0].Labels))
		})

		t.Run("withNestedJSONOverridesFlatJSON", func(t *testing.T) {
			input := modeldecoder.Input{Base: &modelpb.APMEvent{}}
			str := `{"log":{"@timestamp":1662616971000000,"trace.id":"trace-id","transaction.id":"transaction-id","log.logger":"404","log.origin.file.name":"404","log.origin.file.line":404,"log": {"logger": "testLogger","origin": {"file": {"name": "testFile","line":10},"function": "testFunc"}},"log.level":"error","service.name": "404","service.version": "404", "service.environment": "404","service.node.name": "404","service": {"name": "testsvc","version": "v1.2.0","environment": "prod","node": {"name": "testNode"}},"process.therad.name": "404","process": {"thread": {"name": "testThread"}},"event.dataset":"not_accesslog","event":{"dataset":"accesslog"},"labels":{"k":"v"},"error.type": "404","error.message":"404","error.stack_trace":"404","error": {"type": "illegal-argument","message": "illegal argument received","stack_trace": "stack_trace_as_string"}}}`
			dec := decoder.NewJSONDecoder(strings.NewReader(str))
			var batch modelpb.Batch
			require.NoError(t, DecodeNestedLog(dec, &input, &batch))
			require.Len(t, batch, 1)
			assert.Equal(t, "2022-09-08 06:02:51 +0000 UTC", modelpb.ToTime(batch[0].Timestamp).String())
			assert.Equal(t, "trace-id", batch[0].Trace.Id)
			assert.Equal(t, "transaction-id", batch[0].Transaction.Id)
			assert.Equal(t, "error", batch[0].Log.Level)
			assert.Equal(t, "testLogger", batch[0].Log.Logger)
			assert.Equal(t, "testFile", batch[0].Log.Origin.File.Name)
			assert.Equal(t, uint32(10), batch[0].Log.Origin.File.Line)
			assert.Equal(t, "testFunc", batch[0].Log.Origin.FunctionName)
			assert.Equal(t, "testsvc", batch[0].Service.Name)
			assert.Equal(t, "v1.2.0", batch[0].Service.Version)
			assert.Equal(t, "prod", batch[0].Service.Environment)
			assert.Equal(t, "testNode", batch[0].Service.Node.Name)
			assert.Equal(t, "testThread", batch[0].Process.Thread.Name)
			assert.Equal(t, "accesslog", batch[0].Event.Dataset)
			assert.Equal(t, "illegal-argument", batch[0].Error.Type)
			assert.Equal(t, "illegal argument received", batch[0].Error.Message)
			assert.Equal(t, "stack_trace_as_string", batch[0].Error.StackTrace)
			assert.Equal(t, modelpb.Labels{"k": &modelpb.LabelValue{Value: "v"}}, modelpb.Labels(batch[0].Labels))
		})
	})

	t.Run("malformed", func(t *testing.T) {
		input := modeldecoder.Input{}
		var batch modelpb.Batch
		err := DecodeNestedLog(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var batch modelpb.Batch
		err := DecodeNestedLog(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToLogModel(t *testing.T) {
	t.Run("log", func(t *testing.T) {
		var input log
		var out modelpb.APMEvent
		input.Level.Set("warn")
		input.Logger.Set("testlogger")
		input.OriginFileName.Set("testfile")
		input.OriginFileLine.Set(10)
		input.OriginFunction.Set("testfunc")
		mapToLogModel(&input, &out)
		assert.Equal(t, "warn", out.Log.Level)
		assert.Equal(t, "testlogger", out.Log.Logger)
		assert.Equal(t, "testfile", out.Log.Origin.File.Name)
		assert.Equal(t, uint32(10), out.Log.Origin.File.Line)
		assert.Equal(t, "testfunc", out.Log.Origin.FunctionName)
	})

	t.Run("faas", func(t *testing.T) {
		var input log
		var out modelpb.APMEvent
		input.FAAS.ID.Set("faasID")
		input.FAAS.Coldstart.Set(true)
		input.FAAS.Execution.Set("execution")
		input.FAAS.Trigger.Type.Set("http")
		input.FAAS.Trigger.RequestID.Set("abc123")
		input.FAAS.Name.Set("faasName")
		input.FAAS.Version.Set("1.0.0")
		mapToLogModel(&input, &out)
		assert.Equal(t, "faasID", out.Faas.Id)
		assert.True(t, *out.Faas.ColdStart)
		assert.Equal(t, "execution", out.Faas.Execution)
		assert.Equal(t, "http", out.Faas.TriggerType)
		assert.Equal(t, "abc123", out.Faas.TriggerRequestId)
		assert.Equal(t, "faasName", out.Faas.Name)
		assert.Equal(t, "1.0.0", out.Faas.Version)
	})

	t.Run("labels", func(t *testing.T) {
		var input log
		var out modelpb.APMEvent
		input.Labels = map[string]any{
			"str":     "str",
			"bool":    true,
			"float":   1.1,
			"float64": float64(1.1),
		}
		mapToLogModel(&input, &out)
		assert.Equal(t, modelpb.Labels{
			"str":  {Value: "str"},
			"bool": {Value: "true"},
		}, modelpb.Labels(out.Labels))
		assert.Equal(t, modelpb.NumericLabels{
			"float":   {Value: 1.1},
			"float64": {Value: 1.1},
		}, modelpb.NumericLabels(out.NumericLabels))
	})

	t.Run("transaction-id", func(t *testing.T) {
		var input log
		var out modelpb.APMEvent
		input.TransactionID.Set("1234")
		mapToLogModel(&input, &out)
		assert.Equal(t, "1234", out.Transaction.Id)
		assert.Equal(t, "1234", out.Span.Id)
	})

	t.Run("span-id", func(t *testing.T) {
		var input log
		var out modelpb.APMEvent
		input.SpanID.Set("1234")
		mapToLogModel(&input, &out)
		assert.Equal(t, "1234", out.Span.Id)
	})

}
