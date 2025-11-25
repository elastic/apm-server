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
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"go.elastic.co/apm/v2/stacktrace"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-perf/loadgen"
	loadgencfg "github.com/elastic/apm-perf/loadgen/config"
	"github.com/elastic/apm-server/systemtest/benchtest/expvar"
)

const waitInactiveTimeout = 90 * time.Second

// BenchmarkFunc is the benchmark function type accepted by Run.
type BenchmarkFunc func(*testing.B, *rate.Limiter)

const benchmarkFuncPrefix = "Benchmark"

type benchmark struct {
	name string
	f    BenchmarkFunc
}

// getLogger returns a logger that does not depend on b.Log because b.Log does not work in testing.Benchmark.
// See https://github.com/golang/go/issues/32066
// Log to stdout to avoid interfering with benchmark output in stderr.
func getLogger() (*zap.Logger, error) {
	c := zap.NewDevelopmentConfig()
	c.OutputPaths = []string{"stdout"}
	return c.Build()
}

func runBenchmark(f BenchmarkFunc) (testing.BenchmarkResult, bool, bool, error) {
	logger, err := getLogger()
	if err != nil {
		return testing.BenchmarkResult{}, false, false, err
	}
	// Run the benchmark. testing.Benchmark will invoke the function
	// multiple times, but only returns the final result.
	var failed bool
	var skipped bool
	var collector *expvar.Collector
	var reterr error
	result := testing.Benchmark(func(b *testing.B) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var err error
		server := loadgencfg.Config.ServerURL.String()
		collector, err = expvar.StartNewCollector(ctx, server, 100*time.Millisecond, logger)
		if err != nil {
			reterr = fmt.Errorf("expvar.StartNewCollector error: %w", err)
			b.Error(reterr)
			failed = b.Failed()
			return
		}

		limiter := loadgen.GetNewLimiter(loadgencfg.Config.EventRate.Burst, loadgencfg.Config.EventRate.Interval)
		b.ResetTimer()
		signal := make(chan bool)
		// f can panic or call runtime.Goexit, stopping the goroutine.
		// When that happens the function won't return and ok=false will
		// be returned, making the benchmark looks like failure.
		go func() {
			// Signal that we're done whether we return normally
			// or by SkipNow/FailNow's runtime.Goexit.
			defer func() {
				signal <- true
			}()

			f(b, limiter)
		}()
		<-signal
		if !b.Failed() {
			watcher, err := collector.WatchMetric(expvar.ActiveEvents, 0)
			if err != nil {
				reterr = fmt.Errorf("collector.WatchMetric error: %w", err)
				b.Error(reterr)
			} else if status := <-watcher; !status {
				reterr = fmt.Errorf("failed to wait for APM server to be inactive")
				b.Error(reterr)
			}
		}
		failed = b.Failed()
		skipped = b.Skipped()
	})
	if result.Extra != nil {
		addExpvarMetrics(&result, collector, benchConfig.Detailed)
	}
	return result, failed, skipped, reterr
}

func addExpvarMetrics(result *testing.BenchmarkResult, collector *expvar.Collector, detailed bool) {
	result.Bytes = collector.Delta(expvar.Bytes)
	result.MemAllocs = uint64(collector.Delta(expvar.MemAllocs))
	result.MemBytes = uint64(collector.Delta(expvar.MemBytes))
	result.Extra["events/sec"] = float64(collector.Delta(expvar.TotalEvents)) / result.T.Seconds()
	if detailed {
		result.Extra["intake_events_accepted/sec"] = float64(collector.Delta(expvar.IntakeEventsAccepted)) / result.T.Seconds()
		result.Extra["intake_events_errors/sec"] = float64(collector.Delta(expvar.IntakeEventsErrorsInvalid)+collector.Delta(expvar.IntakeEventsErrorsTooLarge)) / result.T.Seconds()
		result.Extra["txs/sec"] = float64(collector.Delta(expvar.TransactionsProcessed)) / result.T.Seconds()
		result.Extra["spans/sec"] = float64(collector.Delta(expvar.SpansProcessed)) / result.T.Seconds()
		result.Extra["metrics/sec"] = float64(collector.Delta(expvar.MetricsProcessed)) / result.T.Seconds()
		result.Extra["errors/sec"] = float64(collector.Delta(expvar.ErrorsProcessed)) / result.T.Seconds()
		result.Extra["gc_cycles"] = float64(collector.Delta(expvar.NumGC))
		result.Extra["max_rss"] = float64(collector.Get(expvar.RSSMemoryBytes).Max)
		result.Extra["max_goroutines"] = float64(collector.Get(expvar.Goroutines).Max)
		result.Extra["max_heap_alloc"] = float64(collector.Get(expvar.HeapAlloc).Max)
		result.Extra["max_heap_objects"] = float64(collector.Get(expvar.HeapObjects).Max)
		result.Extra["mean_available_indexers"] = float64(collector.Get(expvar.AvailableBulkRequests).Mean)
		result.Extra["tbs_lsm_size"] = float64(collector.Get(expvar.TBSLsmSize).Max)
		result.Extra["tbs_vlog_size"] = float64(collector.Get(expvar.TBSVlogSize).Max)
	}

	// Record the number of error responses returned by the server: lower is better.
	errorResponses := collector.Delta(expvar.ErrorElasticResponses) +
		collector.Delta(expvar.ErrorOTLPTracesResponses) +
		collector.Delta(expvar.ErrorOTLPMetricsResponses)
	if detailed || errorResponses > 0 {
		result.Extra["error_responses/sec"] = float64(errorResponses) / result.T.Seconds()
	}
}

func fullBenchmarkName(name string, agents int) string {
	if agents != 1 {
		return fmt.Sprintf("%s-%d", name, agents)
	}
	return name
}

func benchmarkFuncName(f BenchmarkFunc) (string, error) {
	ffunc := runtime.FuncForPC(reflect.ValueOf(f).Pointer())
	if ffunc == nil {
		return "", errors.New("runtime.FuncForPC returned nil")
	}
	fullName := ffunc.Name()
	_, name := stacktrace.SplitFunctionName(fullName)
	if !strings.HasPrefix(name, benchmarkFuncPrefix) {
		return "", fmt.Errorf("benchmark function names must begin with %q (got %q)", fullName, benchmarkFuncPrefix)
	}
	return name, nil
}

// Run runs the given benchmarks according to the flags defined.
//
// Run expects to receive statically-defined functions whose names
// are all prefixed with "Benchmark", like those that are designed
// to work with "go test".
func Run(allBenchmarks ...BenchmarkFunc) error {
	// Set flags in package testing.
	testing.Init()
	if err := flag.Set("test.benchtime", benchConfig.Benchtime.String()); err != nil {
		return err
	}
	// Sets the http.DefaultClient.Transport.TLSClientConfig.InsecureSkipVerify
	// to match the "-secure" flag value.
	verifyTLS := loadgencfg.Config.Secure
	http.DefaultClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !verifyTLS},
	}
	os.Setenv("ELASTIC_APM_VERIFY_SERVER_CERT", fmt.Sprint(verifyTLS))
	var profiles profiles
	if err := profiles.init(); err != nil {
		return err
	}
	defer func() {
		if err := profiles.writeProfiles(); err != nil {
			log.Printf("failed to write profiles: %s", err)
		}
	}()

	matchRE := benchConfig.RunRE
	benchmarks := make([]benchmark, 0, len(allBenchmarks))
	for _, benchmarkFunc := range allBenchmarks {
		name, err := benchmarkFuncName(benchmarkFunc)
		if err != nil {
			return err
		}
		if matchRE == nil || matchRE.MatchString(name) {
			benchmarks = append(benchmarks, benchmark{
				name: name,
				f:    benchmarkFunc,
			})
		}
	}
	sort.Slice(benchmarks, func(i, j int) bool {
		return benchmarks[i].name < benchmarks[j].name
	})

	var maxLen int
	agentsList := benchConfig.AgentsList
	for _, agents := range agentsList {
		for _, benchmark := range benchmarks {
			if n := len(fullBenchmarkName(benchmark.name, agents)); n > maxLen {
				maxLen = n
			}
		}
	}

	// Warm up the APM Server with the specified `-agents`. Only the first
	// value in the list will be used.
	if len(agentsList) > 0 && benchConfig.WarmupTime.Seconds() > 0 {
		agents := agentsList[0]
		serverURL := loadgencfg.Config.ServerURL.String()
		secretToken := loadgencfg.Config.SecretToken
		apiKey := loadgencfg.Config.APIKey
		if err := warmup(agents, benchConfig.WarmupTime, serverURL, secretToken, apiKey); err != nil {
			return fmt.Errorf("warm-up failed with %d agents: %v", agents, err)
		}
	}

	for _, agents := range agentsList {
		runtime.GOMAXPROCS(int(agents))
		for _, benchmark := range benchmarks {
			name := fullBenchmarkName(benchmark.name, agents)
			for i := 0; i < int(benchConfig.Count); i++ {
				profileChan := profiles.record(name)
				result, failed, skipped, err := runBenchmark(benchmark.f)
				if err != nil {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s\n", name)
					return fmt.Errorf("benchmark %q failed: %w", name, err)
				}
				if skipped {
					continue
				}
				if failed {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s\n", name)
					return fmt.Errorf("benchmark %q failed", name)
				} else {
					fmt.Fprintf(os.Stderr, "%-*s\t%s\t%s\n", maxLen, name, result, result.MemString())
				}
				if err := <-profileChan; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// warmup sends events to the remote APM Server using the specified number of
// agents for the specified duration.
func warmup(agents int, duration time.Duration, url, token, apiKey string) error {
	rl := loadgen.GetNewLimiter(loadgencfg.Config.EventRate.Burst, loadgencfg.Config.EventRate.Interval)
	h, err := loadgen.NewEventHandler(loadgen.EventHandlerParams{
		Logger:   zap.NewNop(),
		Protocol: "apm/http",
		Path:     `apm-*.ndjson`,
		URL:      url,
		Token:    token,
		APIKey:   apiKey,
		Limiter:  rl,
	})
	if err != nil {
		return fmt.Errorf("unable to create warm-up handler: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(agents)
	for i := 0; i < agents; i++ {
		go func() {
			defer wg.Done()
			sendErr := h.SendBatchesInLoop(ctx)
			if sendErr != nil && !errors.Is(sendErr, context.DeadlineExceeded) {
				log.Printf("failed to send batches: %v", sendErr)
			}
		}()
	}

	wg.Wait()

	ctx, cancel = context.WithTimeout(context.Background(), waitInactiveTimeout)
	defer cancel()
	if err := expvar.WaitUntilServerInactive(ctx, url); err != nil {
		return fmt.Errorf("received error waiting for server inactive: %w", err)
	}
	return nil
}
