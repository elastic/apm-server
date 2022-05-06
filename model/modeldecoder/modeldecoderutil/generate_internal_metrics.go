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

//go:build tools
// +build tools

package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"text/template"

	"gopkg.in/yaml.v3"
)

func main() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("runtime.Caller failed")
	}
	modelprocessorDir := filepath.Dir(file)
	repoRoot := filepath.Join(modelprocessorDir, "..", "..", "..")

	internalMetricsFieldsDir := filepath.Join(repoRoot, "apmpackage", "apm", "data_stream", "internal_metrics", "fields")
	files, err := filepath.Glob(filepath.Join(internalMetricsFieldsDir, "*_metrics.yml"))
	if err != nil {
		log.Fatal(err)
	}

	metricNames := make(map[string]struct{})
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}

		var fields []field
		if err := yaml.Unmarshal(data, &fields); err != nil {
			log.Fatal(err)
		}
		for _, field := range fields {
			metricNames[field.Name] = struct{}{}
		}
	}

	justMetricNames := make([]string, 0, len(metricNames))
	for name := range metricNames {
		justMetricNames = append(justMetricNames, name)
	}
	sort.Strings(justMetricNames)

	fout, err := os.Create(filepath.Join(modelprocessorDir, "internal_metrics.go"))
	if err != nil {
		log.Fatal(err)
	}
	if err := tmpl.Execute(fout, justMetricNames); err != nil {
		log.Fatal(err)
	}
	if err := fout.Close(); err != nil {
		log.Fatal(err)
	}
}

type field struct {
	Name string
}

var tmpl = template.Must(template.New("").Parse(`
package modeldecoderutil

// IsInternalMetricName reports whether or not a metric name is a known
// internal metric: runtime, system, or process metrics reported by agents.
func IsInternalMetricName(name string) bool {
	switch name {
	{{range . -}}
	case "{{.}}":
		return true
	{{end -}}
	}
	return false
}
`[1:]))
