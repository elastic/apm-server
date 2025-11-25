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
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/pprof/profile"

	loadgencfg "github.com/elastic/apm-perf/loadgen/config"
)

func fetchProfile(urlPath string, duration time.Duration) (*profile.Profile, error) {
	serverURL := loadgencfg.Config.ServerURL.String()
	req, err := http.NewRequest("GET", serverURL+urlPath, nil)
	if err != nil {
		return nil, err
	}
	if duration > 0 {
		query := req.URL.Query()
		query.Set("seconds", strconv.Itoa(int(duration.Seconds())))
		req.URL.RawQuery = query.Encode()

		timeout := duration * 3
		ctx, cancel := context.WithTimeout(req.Context(), timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
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
	if benchConfig.CPUProfile == "" {
		return nil
	}
	// Limit profiling time to random 5% of overall time.
	// This should not seriously affect the profile quality,
	// since we merge the final profile form multiple sources,
	// but prevent profile size from swelling.
	var done bool
	const tickets = 20
	duration := benchConfig.Benchtime / tickets
	for i := range tickets {
		if done || (rand.N(tickets-i)+i+1) < tickets {
			time.Sleep(duration)
			continue
		}
		profile, err := fetchProfile("/debug/pprof/profile", duration)
		if err != nil {
			return fmt.Errorf("failed to fetch CPU profile: %w", err)
		}
		// We don't need the address in the profile, so discard it to reduce the size.
		if err := profile.Aggregate(true, true, true, true, false); err != nil {
			return fmt.Errorf("failed to aggregate CPU profile: %w", err)
		}
		profile = profile.Compact()
		p.cpu = append(p.cpu, profile)
		done = true
	}
	return nil
}

func (p *profiles) recordCumulatives() error {
	if err := p.recordCumulative(benchConfig.MemProfile, "/debug/pprof/heap", &p.heap); err != nil {
		return err
	}
	if err := p.recordCumulative(benchConfig.MutexProfile, "/debug/pprof/mutex", &p.mutex); err != nil {
		return err
	}
	if err := p.recordCumulative(benchConfig.BlockProfile, "/debug/pprof/block", &p.block); err != nil {
		return err
	}
	return nil
}

func (p *profiles) recordCumulative(flag string, urlPath string, out *[]*profile.Profile) error {
	if flag == "" {
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
	if err := p.writeCumulative(benchConfig.MemProfile, p.heap); err != nil {
		return err
	}
	if err := p.writeCumulative(benchConfig.MutexProfile, p.mutex); err != nil {
		return err
	}
	if err := p.writeCumulative(benchConfig.BlockProfile, p.block); err != nil {
		return err
	}
	if err := p.writeDeltas(benchConfig.CPUProfile, p.cpu); err != nil {
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
	w, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer w.Close()
	return merged.WriteUncompressed(w)
}

func (p *profiles) mergeBenchmarkProfiles(profiles []*profile.Profile) (*profile.Profile, error) {
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
