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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const usageTail = `
Path has to lead to any one of these files:
- monitoring-beats.json
- monitoring-beats-mb.json
- metricbeat/module/beat/stats/_meta/fields.yml
- metricbeat/module/beat/_meta/fields.yml
- elastic_agent/data_stream/apm_server_metrics/fields/beat-stats-fields.yml

This program reads apm-server monitoring stats json (from
apm-server:5066/stats) from stdin, and modifies the specified files in-place
to contain the mappings.
`

func main() {
	if err := run(os.Args, os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader) error {
	if len(args) < 2 {
		return fmt.Errorf("no file specified\n\nUsage: %s path_to_file...\n%s", progName(args), usageTail)
	}
	stats, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	for _, path := range args[1:] {
		if err := dispatch(path, stats); err != nil {
			return err
		}
	}
	return nil
}

// dispatch routes a single path to the matching handler. The basename match
// for the JSON templates runs first, then the path-suffix matches for the YAML
// files. The stats-fields suffix must be checked before the root-fields suffix
// because the latter is a prefix of the former.
func dispatch(path string, stats []byte) error {
	switch filepath.Base(path) {
	case "monitoring-beats.json":
		return modifyMonitoringBeats(path, stats)
	case "monitoring-beats-mb.json":
		return modifyMonitoringBeatsMB(path, stats)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", path, err)
	}
	slash := filepath.ToSlash(abs)
	switch {
	case strings.HasSuffix(slash, "/metricbeat/module/beat/stats/_meta/fields.yml"):
		return modifyBeatStats(path, stats)
	case strings.HasSuffix(slash, "/metricbeat/module/beat/_meta/fields.yml"):
		return modifyBeatRoot(path, stats)
	case strings.HasSuffix(slash, "/elastic_agent/data_stream/apm_server_metrics/fields/beat-stats-fields.yml"):
		return modifyEAFields(path, stats)
	}
	return fmt.Errorf("path %s does not match any supported file", path)
}

func progName(args []string) string {
	if len(args) == 0 {
		return "stats-to-mapping"
	}
	return args[0]
}
