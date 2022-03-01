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

package benchtest

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
)

// TODO(axw) reuse apmservertest.Expvar, expose function(s) for fetching
// from APM Server given a URL.

type expvar struct {
	runtime.MemStats `json:"memstats"`
	LibbeatStats
	ElasticResponseStats
	OTLPResponseStats

	// UncompressedBytes holds the number of bytes of uncompressed
	// data that the server has read from the Elastic APM events
	// intake endpoint.
	//
	// TODO(axw) instrument the net/http.Transport to count bytes
	// transferred, so we can measure for OTLP and Jaeger too.
	// Alternatively, implement an in-memory reverse proxy that
	// does the same.
	UncompressedBytes int64 `json:"apm-server.decoder.uncompressed.bytes"`
}

type ElasticResponseStats struct {
	TotalElasticResponses int64 `json:"apm-server.server.response.count"`
	ErrorElasticResponses int64 `json:"apm-server.server.response.errors.count"`
	TransactionsProcessed int64 `json:"apm-server.processor.transaction.transformations"`
	SpansProcessed        int64 `json:"apm-server.processor.span.transformations"`
	MetricsProcessed      int64 `json:"apm-server.processor.metric.transformations"`
	ErrorsProcessed       int64 `json:"apm-server.processor.error.transformations"`
}

type OTLPResponseStats struct {
	TotalOTLPMetricsResponses int64 `json:"apm-server.otlp.grpc.metrics.response.count"`
	ErrorOTLPMetricsResponses int64 `json:"apm-server.otlp.grpc.metrics.response.errors.count"`
	TotalOTLPTracesResponses  int64 `json:"apm-server.otlp.grpc.traces.response.count"`
	ErrorOTLPTracesResponses  int64 `json:"apm-server.otlp.grpc.traces.response.errors.count"`
}

type LibbeatStats struct {
	ActiveEvents int64 `json:"libbeat.output.events.active"`
	TotalEvents  int64 `json:"libbeat.output.events.total"`
}

func queryExpvar(out *expvar) error {
	req, err := http.NewRequest("GET", *server+"/debug/vars", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

// WaitUntilServerInactive blocks until one of the conditions occurs:
// * APM Server output active events are lower than `warmupInactive`.
// * HTTP call returns with an error
// * Context is done.
func WaitUntilServerInactive(ctx context.Context) error {
	errChan := make(chan error)
	go func() {
		var result expvar
		defer close(errChan)
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}
			if err := queryExpvar(&result); err != nil {
				errChan <- err
				return
			}
			if result.ActiveEvents < int64(*inactiveThreshold) {
				return
			}
		}
	}()

	return <-errChan
}
