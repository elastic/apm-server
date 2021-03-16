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

package systemtest_test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/apmtest"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func TestAPMServerGRPCRequestLoggingValid(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Jaeger = &apmservertest.JaegerConfig{
		GRPCEnabled: true,
		GRPCHost:    "localhost:0",
	}
	err := srv.Start()
	require.NoError(t, err)
	addr := srv.JaegerGRPCAddr
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := api_v2.NewCollectorServiceClient(conn)
	request, err := decodeJaegerPostSpansRequest("../testdata/jaeger/batch_0.json")
	require.NoError(t, err)
	_, err = client.PostSpans(context.Background(), request)
	require.NoError(t, err)

<<<<<<< HEAD
	tracerProvider := newOTLPTracerProvider(newOTLPExporter(t, srv, otlpgrpc.WithHeaders(map[string]string{
		"Authorization": "Bearer abc123",
	})))
	err = sendOTLPTrace(context.Background(), tracerProvider)
=======
	err = sendOTLPTrace(context.Background(), srv, otlpgrpc.WithHeaders(map[string]string{"Authorization": "Bearer abc123"}))
>>>>>>> 17433dac9... add logging to jaeger and otlp gRPC calls (#4934)
	require.NoError(t, err)

	srv.Close()

	var foundGRPC, foundJaeger bool
	for _, entry := range srv.Logs.All() {
		if entry.Logger == "beater.grpc" {
			require.Equal(t, "/opentelemetry.proto.collector.trace.v1.TraceService/Export", entry.Fields["grpc.request.method"])
			require.Equal(t, "OK", entry.Fields["grpc.response.status_code"])
			foundGRPC = true
		}
		if entry.Logger == "beater.jaeger" {
			require.Equal(t, "/jaeger.api_v2.CollectorService/PostSpans", entry.Fields["grpc.request.method"])
			require.Equal(t, "OK", entry.Fields["grpc.response.status_code"])
			foundJaeger = true
		}
	}
	require.True(t, foundGRPC)
	require.True(t, foundJaeger)
}

func TestAPMServerRequestLoggingValid(t *testing.T) {
	srv := apmservertest.NewServer(t)
	eventsURL := srv.URL + "/intake/v2/events"

	// Send a request to the server with a single valid event.
	tracer := srv.Tracer()
	tracer.StartTransaction("name", "type").End()
	tracer.Flush(nil)

	// Send a second, invalid request with JSON that does not validate against the schema.
	invalidBody := strings.NewReader(validMetadataJSON() + "\n{\"transaction\":{}}\n")
	resp, err := http.Post(eventsURL, "application/x-ndjson", invalidBody)
	require.NoError(t, err)
	resp.Body.Close()

	// Send a third, invalid request with an excessively large event.
	overlargeBody := strings.NewReader(fmt.Sprintf(
		"%s\n{\"transaction\":{\"name\":%q}}\n",
		validMetadataJSON(), strings.Repeat("a", 1024*1024),
	))
	resp, err = http.Post(eventsURL, "application/x-ndjson", overlargeBody)
	require.NoError(t, err)
	io.Copy(ioutil.Discard, resp.Body) // Wait for server to respond
	resp.Body.Close()

	type requestEntry struct {
		level      zapcore.Level
		message    string
		statusCode int
	}
	var requestEntries []requestEntry
	var logEntries []apmservertest.LogEntry // corresponding raw log entries

	srv.Close()
	for _, entry := range srv.Logs.All() {
		if entry.Logger == "request" && entry.Fields["url.original"] == "/intake/v2/events" {
			statusCode, _ := entry.Fields["http.response.status_code"].(float64)
			logEntries = append(logEntries, entry)
			requestEntries = append(requestEntries, requestEntry{
				level:      entry.Level,
				message:    entry.Message,
				statusCode: int(statusCode),
			})
		}
	}

	assert.Equal(t, []requestEntry{{
		level:      zapcore.InfoLevel,
		message:    "request accepted",
		statusCode: 202,
	}, {
		level:      zapcore.ErrorLevel,
		message:    "data validation error",
		statusCode: 400,
	}, {
		level:      zapcore.ErrorLevel,
		message:    "request body too large",
		statusCode: 400,
	}}, requestEntries)

	assert.NotContains(t, logEntries[0].Fields, "error")
	assert.Regexp(t, "validation error: 'transaction' required", logEntries[1].Fields["error.message"])
	assert.Equal(t, "event exceeded the permitted size.", logEntries[2].Fields["error.message"])
}

// validMetadataJSON returns valid JSON-encoded metadata,
// using the Go agent to generate it. This should be used
// when we don't care about the metadata content, to avoid
// having to keep incidental test data up-to-date.
func validMetadataJSON() string {
	tracer := apmtest.NewRecordingTracer()
	tracer.StartTransaction("name", "type").End()
	tracer.Flush(nil)
	system, process, service, labels := tracer.Metadata()

	var w fastjson.Writer
	w.RawString(`{"metadata":{`)
	w.RawString(`"system":`)
	system.MarshalFastJSON(&w)
	w.RawString(`,"process":`)
	process.MarshalFastJSON(&w)
	w.RawString(`,"service":`)
	service.MarshalFastJSON(&w)
	if len(labels) > 0 {
		w.RawString(`,"labels":`)
		labels.MarshalFastJSON(&w)
	}
	w.RawString("}}")
	return string(w.Bytes())
}
