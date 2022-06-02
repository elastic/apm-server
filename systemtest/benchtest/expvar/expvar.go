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

package expvar

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
)

const (
	cloudProxyHeader = "x-found-handling-instance"
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
	UncompressedBytes     int64 `json:"apm-server.decoder.uncompressed.bytes"`
	AvailableBulkRequests int64 `json:"output.elasticsearch.bulk_requests.available"`
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
	ActiveEvents   int64 `json:"libbeat.output.events.active"`
	TotalEvents    int64 `json:"libbeat.output.events.total"`
	Goroutines     int64 `json:"beat.runtime.goroutines"`
	RSSMemoryBytes int64 `json:"beat.memstats.rss"`
}

func queryExpvar(ctx context.Context, out *expvar, srv string) error {
	req, err := http.NewRequest("GET", srv+"/debug/vars", nil)
	if err != nil {
		return err
	}
	req.WithContext(ctx)
	req.Header.Set("Accept", "application/json")

	agg := make(map[string]expvar)
	var seen int
	const timesSeen = 10
	for {
		var tmp expvar
		// NOTE(marclop) we could also aggregate based on the beats ephemeral id.
		// However, that has the drawback of not being to replace a node when it
		// restarts.
		// For example, if we push too hard and it falls out of the LB, the beats
		// ID will change if the APM Server process is restarted, resulting in
		// some stats never being updated.
		id, err := doExpvar(req, &tmp)
		if err != nil {
			return err
		}
		if _, ok := agg[id]; ok {
			seen++
		}
		agg[id] = tmp
		// We must ensure that we have made enough requests to the remote APM
		// Server to guarantee with a degree of certainty that all the remote
		// APM Servers metrics have been queried.
		if seen > timesSeen*len(agg) {
			break
		}
	}

	var result expvar
	for _, s := range agg {
		aggregateMemStats(s.MemStats, &result.MemStats)
		aggregateResponseStats(s.ElasticResponseStats, &result.ElasticResponseStats)
		aggregateOTLPResponseStats(s.OTLPResponseStats, &result.OTLPResponseStats)
		aggregateLibbeatStats(s.LibbeatStats, &result.LibbeatStats)
		result.UncompressedBytes += s.UncompressedBytes
		result.AvailableBulkRequests += s.AvailableBulkRequests
	}
	*out = result
	return nil
}

func doExpvar(req *http.Request, out *expvar) (string, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	id := resp.Header.Get(cloudProxyHeader)
	err = json.NewDecoder(resp.Body).Decode(out)
	return id, err
}

// WaitUntilServerInactive blocks until one of the conditions occurs:
// * APM Server is inactive (has no "active" events on the output buffer).
// * HTTP call returns with an error
// * Context is done.
func WaitUntilServerInactive(ctx context.Context, server string) error {
	result := expvar{LibbeatStats: LibbeatStats{ActiveEvents: 1}}
	for result.ActiveEvents > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := queryExpvar(ctx, &result, server); err != nil {
				return err
			}
		}
	}
	return nil
}

// NOTE(marclop): There are some fields that aren't being aggregated
// like the circular buffers PauseNS, PauseEnd, BySize, and booleans
// (DebugGC, EnableGC).
func aggregateMemStats(from runtime.MemStats, to *runtime.MemStats) {
	to.Alloc += from.Alloc
	to.TotalAlloc += from.TotalAlloc
	to.Sys += from.Sys
	to.Lookups += from.Lookups
	to.Mallocs += from.Mallocs
	to.Frees += from.Frees
	to.HeapAlloc += from.HeapAlloc
	to.HeapSys += from.HeapSys
	to.HeapIdle += from.HeapIdle
	to.HeapInuse += from.HeapInuse
	to.HeapReleased += from.HeapReleased
	to.HeapObjects += from.HeapObjects
	to.StackInuse += from.StackInuse
	to.StackSys += from.StackSys
	to.MSpanInuse += from.MSpanInuse
	to.MSpanSys += from.MSpanSys
	to.MCacheInuse += from.MCacheInuse
	to.MCacheSys += from.MCacheSys
	to.BuckHashSys += from.BuckHashSys
	to.GCSys += from.GCSys
	to.OtherSys += from.OtherSys
	to.NextGC += from.NextGC
	to.LastGC += from.LastGC
	to.PauseTotalNs += from.PauseTotalNs
	to.NumGC += from.NumGC
	to.NumForcedGC += from.NumForcedGC
	to.GCCPUFraction += from.GCCPUFraction
}

func aggregateLibbeatStats(from LibbeatStats, to *LibbeatStats) {
	to.ActiveEvents += from.ActiveEvents
	to.TotalEvents += from.TotalEvents
	to.Goroutines += from.Goroutines
	to.RSSMemoryBytes += from.RSSMemoryBytes
}

func aggregateResponseStats(from ElasticResponseStats, to *ElasticResponseStats) {
	to.ErrorElasticResponses += from.ErrorElasticResponses
	to.ErrorsProcessed += from.ErrorsProcessed
	to.MetricsProcessed += from.MetricsProcessed
	to.SpansProcessed += from.SpansProcessed
	to.TransactionsProcessed += from.TransactionsProcessed
	to.TotalElasticResponses += from.TotalElasticResponses
}

func aggregateOTLPResponseStats(from OTLPResponseStats, to *OTLPResponseStats) {
	to.TotalOTLPMetricsResponses += from.TotalOTLPMetricsResponses
	to.TotalOTLPTracesResponses += from.TotalOTLPTracesResponses
	to.ErrorOTLPTracesResponses += from.ErrorOTLPTracesResponses
	to.ErrorOTLPMetricsResponses += from.ErrorOTLPMetricsResponses
}
