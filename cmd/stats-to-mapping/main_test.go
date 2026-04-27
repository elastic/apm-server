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
	"os"
	"path/filepath"
	"testing"
)

// TestGolden routes every supported file type through dispatch and compares
// the resulting bytes against the committed golden. dirPath places the
// input fixture at a path that dispatch's basename / path-suffix rules will
// match. On mismatch, the actual output is written to a temp path and the
// path is reported so the developer can diff it against the golden.
func TestGolden(t *testing.T) {
	stats, err := os.ReadFile("testdata/stats.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, input, golden, dirPath string
	}{
		{
			name:    "monitoring-beats.json",
			input:   "testdata/inputs/monitoring-beats.json",
			golden:  "testdata/golden/monitoring-beats.json",
			dirPath: "monitoring-beats.json",
		},
		{
			name:    "monitoring-beats-mb.json",
			input:   "testdata/inputs/monitoring-beats-mb.json",
			golden:  "testdata/golden/monitoring-beats-mb.json",
			dirPath: "monitoring-beats-mb.json",
		},
		{
			name:    "metricbeat beat root fields.yml",
			input:   "testdata/inputs/beat-root-fields.yml",
			golden:  "testdata/golden/beat-root-fields.yml",
			dirPath: "metricbeat/module/beat/_meta/fields.yml",
		},
		{
			name:    "metricbeat beat stats fields.yml",
			input:   "testdata/inputs/beat-stats-fields.yml",
			golden:  "testdata/golden/beat-stats-fields.yml",
			dirPath: "metricbeat/module/beat/stats/_meta/fields.yml",
		},
		{
			name:    "elastic_agent beat-stats-fields.yml",
			input:   "testdata/inputs/ea-beat-stats-fields.yml",
			golden:  "testdata/golden/ea-beat-stats-fields.yml",
			dirPath: "elastic_agent/data_stream/apm_server_metrics/fields/beat-stats-fields.yml",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workPath := filepath.Join(t.TempDir(), tc.dirPath)
			if err := os.MkdirAll(filepath.Dir(workPath), 0o755); err != nil {
				t.Fatal(err)
			}
			input, err := os.ReadFile(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(workPath, input, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := dispatch(workPath, stats); err != nil {
				t.Fatalf("dispatch: %v", err)
			}
			got, err := os.ReadFile(workPath)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(tc.golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				actual := filepath.Join(t.TempDir(), filepath.Base(tc.golden)+".actual")
				_ = os.WriteFile(actual, got, 0o644)
				t.Fatalf("output differs from golden\n  golden: %s\n  actual: %s\n  diff with: diff %s %s",
					tc.golden, actual, tc.golden, actual)
			}
		})
	}
}
