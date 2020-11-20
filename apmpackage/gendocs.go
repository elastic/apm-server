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

package apmpackage

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

func GenerateDocs(inputFields map[string][]FieldDefinition, version string) {
	// TODO sort alphabetically and add datasets (from base-fields)
	data := FlattenAPMFields{
		Traces:             flatten("", inputFields["traces"]),
		Metrics:            flatten("", inputFields["metrics"]),
		Logs:               flatten("", inputFields["logs"]),
		TransactionExample: loadExample("transactions.json"),
		SpanExample:        loadExample("spans.json"),
		MetricsExample:     loadExample("metricsets.json"),
		ErrorExample:       loadExample("errors.json"),
	}
	t := template.New("apmpackage/docs/README.template.md")
	tmpl, err := t.Funcs(map[string]interface{}{
		"Trim": strings.TrimSpace,
	}).ParseFiles("apmpackage/docs/README.template.md")
	if err != nil {
		panic(err)
	}
	path := filepath.Join("apmpackage/apm/", version, "/docs/README.md")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	err = tmpl.ExecuteTemplate(file, "README.template.md", data)
	if err != nil {
		panic(err)
	}
}

type FlattenAPMFields struct {
	Traces             []FieldDefinition
	Metrics            []FieldDefinition
	Logs               []FieldDefinition
	TransactionExample string
	SpanExample        string
	MetricsExample     string
	ErrorExample       string
}

func flatten(name string, fs []FieldDefinition) []FieldDefinition {
	var ret []FieldDefinition
	for _, f := range fs {
		if name != "" {
			f.Name = name + "." + f.Name
		}
		if f.Type == "group" {
			ret = append(ret, flatten(f.Name, f.Fields)...)
		} else {
			ret = append(ret, f)
		}
	}
	return ret
}

func loadExample(file string) string {
	in, err := ioutil.ReadFile(path.Join("docs/data/elasticsearch/generated/", file))
	if err != nil {
		panic(err)
	}
	var aux []map[string]interface{}
	err = json.Unmarshal(in, &aux)
	if err != nil {
		panic(err)
	}
	out, err := json.MarshalIndent(aux[0], "", "  ")
	if err != nil {
		panic(err)
	}
	return string(out)
}
