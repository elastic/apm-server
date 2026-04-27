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
			"monitoring-beats.json",
			"testdata/inputs/monitoring-beats.json",
			"testdata/golden/monitoring-beats.json",
			"monitoring-beats.json",
		},
		{
			"monitoring-beats-mb.json",
			"testdata/inputs/monitoring-beats-mb.json",
			"testdata/golden/monitoring-beats-mb.json",
			"monitoring-beats-mb.json",
		},
		{
			"metricbeat beat root fields.yml",
			"testdata/inputs/beat-root-fields.yml",
			"testdata/golden/beat-root-fields.yml",
			"metricbeat/module/beat/_meta/fields.yml",
		},
		{
			"metricbeat beat stats fields.yml",
			"testdata/inputs/beat-stats-fields.yml",
			"testdata/golden/beat-stats-fields.yml",
			"metricbeat/module/beat/stats/_meta/fields.yml",
		},
		{
			"elastic_agent beat-fields.yml",
			"testdata/inputs/ea-beat-fields.yml",
			"testdata/golden/ea-beat-fields.yml",
			"elastic_agent/data_stream/apm_server_metrics/fields/beat-fields.yml",
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
