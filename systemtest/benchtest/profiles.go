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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/pprof/profile"
)

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
