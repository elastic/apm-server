// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestGolden exercises every supported file type. For each, the input fixture
// is copied into a t.TempDir, the relevant handler is invoked with stats.json
// as input bytes, and the resulting file is compared byte-for-byte to the
// committed golden produced by the upstream Python script. On mismatch, the
// actual output is written next to the test binary so the developer can run a
// diff outside the test process.
func TestGolden(t *testing.T) {
	stats, err := os.ReadFile("testdata/stats.json")
	if err != nil {
		t.Fatalf("reading stats fixture: %v", err)
	}

	cases := []struct {
		name    string
		input   string
		golden  string
		dirPath string // path layout required by dispatch (relative to t.TempDir)
		handler func(path string, stats []byte) error
		// useDispatch routes through main.dispatch instead of calling the
		// handler directly, exercising the path-suffix matching for YAML files.
		useDispatch bool
	}{
		{
			name:    "monitoring-beats.json",
			input:   "testdata/inputs/monitoring-beats.json",
			golden:  "testdata/golden/monitoring-beats.json",
			dirPath: "monitoring-beats.json",
			handler: modifyMonitoringBeats,
		},
		{
			name:    "monitoring-beats-mb.json",
			input:   "testdata/inputs/monitoring-beats-mb.json",
			golden:  "testdata/golden/monitoring-beats-mb.json",
			dirPath: "monitoring-beats-mb.json",
			handler: modifyMonitoringBeatsMB,
		},
		{
			name:        "metricbeat beat root fields.yml",
			input:       "testdata/inputs/beat-root-fields.yml",
			golden:      "testdata/golden/beat-root-fields.yml",
			dirPath:     "metricbeat/module/beat/_meta/fields.yml",
			useDispatch: true,
		},
		{
			name:        "metricbeat beat stats fields.yml",
			input:       "testdata/inputs/beat-stats-fields.yml",
			golden:      "testdata/golden/beat-stats-fields.yml",
			dirPath:     "metricbeat/module/beat/stats/_meta/fields.yml",
			useDispatch: true,
		},
		{
			name:        "elastic_agent beat-fields.yml",
			input:       "testdata/inputs/ea-beat-fields.yml",
			golden:      "testdata/golden/ea-beat-fields.yml",
			dirPath:     "elastic_agent/data_stream/apm_server_metrics/fields/beat-fields.yml",
			useDispatch: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workdir := t.TempDir()
			workPath := filepath.Join(workdir, tc.dirPath)
			if err := os.MkdirAll(filepath.Dir(workPath), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			input, err := os.ReadFile(tc.input)
			if err != nil {
				t.Fatalf("reading input: %v", err)
			}
			if err := os.WriteFile(workPath, input, 0o644); err != nil {
				t.Fatalf("seeding input: %v", err)
			}
			if tc.useDispatch {
				if err := dispatch(workPath, stats); err != nil {
					t.Fatalf("dispatch: %v", err)
				}
			} else {
				if err := tc.handler(workPath, stats); err != nil {
					t.Fatalf("handler: %v", err)
				}
			}
			got, err := os.ReadFile(workPath)
			if err != nil {
				t.Fatalf("reading output: %v", err)
			}
			want, err := os.ReadFile(tc.golden)
			if err != nil {
				t.Fatalf("reading golden: %v", err)
			}
			if !bytes.Equal(got, want) {
				artifact := filepath.Join(t.TempDir(), filepath.Base(tc.golden)+".actual")
				_ = os.WriteFile(artifact, got, 0o644)
				t.Fatalf("byte mismatch.\nwrote actual output to %s\ndiff against %s\n%s",
					artifact, tc.golden, firstDiff(got, want))
			}
		})
	}
}

// firstDiff returns a short message identifying the first byte where got and
// want diverge, with surrounding context.
func firstDiff(got, want []byte) string {
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		if got[i] != want[i] {
			return diffContext(got, want, i)
		}
	}
	if len(got) != len(want) {
		return diffContext(got, want, n)
	}
	return ""
}

func diffContext(got, want []byte, i int) string {
	const radius = 60
	lo := max(0, i-radius)
	return fmt.Sprintf("first byte differs at offset %d\n  got:  %s\n  want: %s",
		i,
		safeSlice(got, lo, i+radius),
		safeSlice(want, lo, i+radius),
	)
}

func safeSlice(b []byte, lo, hi int) string {
	if lo < 0 {
		lo = 0
	}
	if hi > len(b) {
		hi = len(b)
	}
	if lo > hi {
		return ""
	}
	return string(b[lo:hi])
}
