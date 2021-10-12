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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/pprof/profile"

	"go.elastic.co/apm"
)

func init() {
	apm.DefaultTracer.Close()
}

var (
	server        = flag.String("server", "http://localhost:8200", "apm-server URL")
	count         = flag.Uint("count", 1, "run benchmarks `n` times")
	agentsListStr = flag.String("agents", "1", "comma-separated `list` of agent counts to run each benchmark with")
	benchtime     = flag.Duration("benchtime", time.Second, "run each benchmark for duration `d`")
	//match = flag.String("run", "", "run only benchmarks matching `regexp`")

	cpuprofile   = flag.String("cpuprofile", "", "Write a CPU profile to the specified file before exiting.")
	memprofile   = flag.String("memprofile", "", "Write an allocation profile to the file  before exiting.")
	mutexprofile = flag.String("mutexprofile", "", "Write a mutex contention profile to the file  before exiting.")
	blockprofile = flag.String("blockprofile", "", "Write a goroutine blocking profile to the file before exiting.")

	agentsList []int
)

type expvar struct {
	runtime.MemStats `json:"memstats"`
	LibbeatStats

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

type LibbeatStats struct {
	TotalEvents int64 `json:"libbeat.output.events.total"`
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

func fetchProfile(urlPath string, duration time.Duration) (*profile.Profile, error) {
	req, err := http.NewRequest("GET", *server+urlPath, nil)
	if err != nil {
		return nil, err
	}
	if duration > 0 {
		query := req.URL.Query()
		query.Set("seconds", strconv.Itoa(int(duration.Seconds())))
		req.URL.RawQuery = query.Encode()

		timeout := time.Duration(float64(duration) * 1.5)
		ctx := req.Context()
		ctx, cancel := context.WithTimeout(req.Context(), timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch profile (%s): %s", resp.Status, body)
	}
	return profile.Parse(resp.Body)
}

type profiles struct {
	benchmarkNames []string
	cpu            []*profile.Profile
	heap           []*profile.Profile
	mutex          []*profile.Profile
	block          []*profile.Profile
}

func (p *profiles) init() error {
	return p.recordCumulatives()
}

func (p *profiles) record(benchmarkName string) <-chan error {
	record := func() error {
		p.benchmarkNames = append(p.benchmarkNames, benchmarkName)
		if err := p.recordCPU(); err != nil {
			return err
		}
		return p.recordCumulatives()
	}
	ch := make(chan error, 1)
	go func() { ch <- record() }()
	return ch
}

func (p *profiles) recordCPU() error {
	if *cpuprofile == "" {
		return nil
	}
	duration := 2 * (*benchtime)
	profile, err := fetchProfile("/debug/pprof/profile", duration)
	if err != nil {
		return fmt.Errorf("failed to fetch CPU profile: %w", err)
	}
	p.cpu = append(p.cpu, profile)
	return nil
}

func (p *profiles) recordCumulatives() error {
	if err := p.recordCumulative(memprofile, "/debug/pprof/heap", &p.heap); err != nil {
		return err
	}
	if err := p.recordCumulative(mutexprofile, "/debug/pprof/mutex", &p.mutex); err != nil {
		return err
	}
	if err := p.recordCumulative(blockprofile, "/debug/pprof/block", &p.block); err != nil {
		return err
	}
	return nil
}

func (p *profiles) recordCumulative(flag *string, urlPath string, out *[]*profile.Profile) error {
	if *flag == "" {
		return nil
	}
	profile, err := fetchProfile(urlPath, 0)
	if err != nil {
		return err
	}
	*out = append(*out, profile)
	return nil
}

func (p *profiles) writeProfiles() error {
	if err := p.writeCumulative(*memprofile, p.heap); err != nil {
		return err
	}
	if err := p.writeCumulative(*mutexprofile, p.mutex); err != nil {
		return err
	}
	if err := p.writeCumulative(*blockprofile, p.block); err != nil {
		return err
	}
	if err := p.writeDeltas(*cpuprofile, p.cpu); err != nil {
		return err
	}
	return nil
}

func (p *profiles) writeCumulative(filename string, cumulative []*profile.Profile) error {
	if len(cumulative) == 0 {
		return nil
	}
	p0 := cumulative[0]
	deltas := make([]*profile.Profile, len(cumulative)-1)
	for i, p1 := range cumulative[1:] {
		delta, err := computeDeltaProfile(p0, p1)
		if err != nil {
			return err
		}
		deltas[i] = delta
		p0 = p1
	}
	return p.writeDeltas(filename, deltas)
}

func (p *profiles) writeDeltas(filename string, deltas []*profile.Profile) error {
	if len(deltas) == 0 {
		return nil
	}
	merged, err := p.mergeBenchmarkProfiles(deltas)
	if err != nil {
		return err
	}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return merged.Write(f)
}

func (p *profiles) mergeBenchmarkProfiles(profiles []*profile.Profile) (*profile.Profile, error) {
	for i, profile := range profiles {
		benchmarkName := p.benchmarkNames[i]
		profile.SetLabel("benchmark", []string{benchmarkName})
	}
	merged, err := profile.Merge(profiles)
	if err != nil {
		return nil, fmt.Errorf("error merging profiles: %w", err)
	}
	return merged, nil
}

func computeDeltaProfile(p0, p1 *profile.Profile) (*profile.Profile, error) {
	p0.Scale(-1)
	defer p0.Scale(-1) // return to initial state

	merged, err := profile.Merge([]*profile.Profile{p0, p1})
	if err != nil {
		return nil, fmt.Errorf("error computing delta profile: %w", err)
	}
	merged.TimeNanos = p1.TimeNanos
	merged.DurationNanos = p1.TimeNanos - p0.TimeNanos
	return merged, nil
}

func parseFlags() error {
	flag.Parse()

	// Parse -agents.
	for _, val := range strings.Split(*agentsListStr, ",") {
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid value %q for -agents\n", val)
		}
		agentsList = append(agentsList, n)
	}

	// Set flags in package testing.
	testing.Init()
	if err := flag.Set("test.benchtime", benchtime.String()); err != nil {
		return err
	}
	return nil
}

func Benchmark(f func(b *testing.B)) (testing.BenchmarkResult, bool, error) {
	var before expvar
	if err := queryExpvar(&before); err != nil {
		return testing.BenchmarkResult{}, false, err
	}

	// Run the benchmark.
	var ok bool
	result := testing.Benchmark(func(b *testing.B) {
		f(b)
		ok = !b.Failed()
	})

	var after expvar
	if err := queryExpvar(&after); err != nil {
		return testing.BenchmarkResult{}, false, err
	}
	result.MemAllocs = after.MemStats.Mallocs - before.MemStats.Mallocs
	result.MemBytes = after.MemStats.TotalAlloc - before.MemStats.TotalAlloc
	result.Bytes = after.UncompressedBytes - before.UncompressedBytes
	if result.Extra == nil {
		result.Extra = make(map[string]float64)
	}
	result.Extra["events/sec"] = float64(after.TotalEvents-before.TotalEvents) / result.T.Seconds()
	return result, ok, nil
}

func fullBenchmarkName(name string, agents int) string {
	if agents != 1 {
		return fmt.Sprintf("%s-%d", name, agents)
	}
	return name
}

func main() {
	if err := parseFlags(); err != nil {
		log.Fatal(err)
	}

	var profiles profiles
	if err := profiles.init(); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := profiles.writeProfiles(); err != nil {
			log.Fatal(err)
		}
	}()

	type benchmark struct {
		name      string
		benchFunc func(*testing.B)
	}
	benchmarks := []benchmark{
		{"1_Transaction", benchmark1Transaction},
		//{"100_5Spans_5Frames", benchmark100_5_5_Spans},
		//{"100_15Spans_15Frames", benchmark100_15_15_Spans},
		//{"100_30Spans_30Frames", benchmark100_30_30_Spans},
		//{"100_5Error_5Frames", benchmark100_5_5_Errors},
		//{"100_10Error_10Frames", benchmark100_10_10_Errors},
		//{"100_15Error_15Frames", benchmark100_15_15_Errors},
		//{"100_1Error_30Frames", benchmark100_1_30_Errors},
	}

	var maxLen int
	for _, agents := range agentsList {
		for _, benchmark := range benchmarks {
			if n := len(fullBenchmarkName(benchmark.name, agents)); n > maxLen {
				maxLen = n
			}
		}
	}
	for _, agents := range agentsList {
		runtime.GOMAXPROCS(int(agents))
		for _, benchmark := range benchmarks {
			name := fullBenchmarkName(benchmark.name, agents)
			for i := 0; i < int(*count); i++ {
				profileChan := profiles.record(name)
				result, ok, err := Benchmark(benchmark.benchFunc)
				if err != nil {
					log.Fatal(err)
				}
				if !ok {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s\n", name)
				} else {
					fmt.Fprintf(os.Stdout,"%d\n", global_count)
					fmt.Fprintf(os.Stderr, "%-*s\t%s\t%s\n", maxLen, name, result, result.MemString())
				}
				if err := <-profileChan; err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}
