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
	"log"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"

	"github.com/elastic/apm-data/model/modelprocessor"
)

type field struct {
	Name string
}

func main() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")

	internalMetricsFieldsDir := filepath.Join(repoRoot, "apmpackage", "apm", "data_stream", "internal_metrics", "fields")
	files, err := filepath.Glob(filepath.Join(internalMetricsFieldsDir, "*_metrics.yml"))
	if err != nil {
		log.Fatal(err)
	}
	var msg []string
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
			if !modelprocessor.IsInternalMetricName(field.Name) {
				p, err := filepath.Rel(repoRoot, file)
				if err != nil {
					p = file
				}
				msg = append(msg, fmt.Sprintf("%s: field '%s'",
					p, field.Name,
				))
			}
		}
	}

	if len(msg) > 0 {
		fmt.Println(`
found extra internal metrics in apmpackage not present in github.com/elastic/apm-data internally defined metrics
please update the upstream list of internal metrics:`[1:])
		for _, m := range msg {
			fmt.Println("-", m)
		}
		os.Exit(1)
	}
}
