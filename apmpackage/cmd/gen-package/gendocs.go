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
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"text/template"
)

func generateDocs(inputFields map[string][]field, version string) {
	data := docsData{
		Traces:             prepareFields(inputFields, version, "traces"),
		Metrics:            prepareFields(inputFields, version, "metrics"),
		Logs:               prepareFields(inputFields, version, "logs"),
		TransactionExample: loadExample("transactions.json"),
		SpanExample:        loadExample("spans.json"),
		MetricsExample:     loadExample("metricsets.json"),
		ErrorExample:       loadExample("errors.json"),
	}
	t := template.New(docsTemplateFilePath)
	tmpl, err := t.Funcs(map[string]interface{}{
		"Trim": strings.TrimSpace,
	}).ParseFiles(docsTemplateFilePath)
	if err != nil {
		panic(err)
	}
	path := docsFilePath(version)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	err = tmpl.ExecuteTemplate(file, "README.template.md", data)
	if err != nil {
		panic(err)
	}
}

type docsData struct {
	Traces             []field
	Metrics            []field
	Logs               []field
	TransactionExample string
	SpanExample        string
	MetricsExample     string
	ErrorExample       string
}

func prepareFields(inputFields map[string][]field, version, streamType string) []field {
	extend := func(fs []field) []field {
		var baseFields []field
		for _, f := range loadFieldsFile(baseFieldsFilePath(version, streamType)) {
			f.IsECS = true
			baseFields = append(baseFields, f)
		}
		fs = append(baseFields, fs...)
		return fs
	}
	return extend(order(flatten("", inputFields[streamType])))
}

func order(fs []field) []field {
	sort.Slice(fs, func(i, j int) bool {
		return fs[i].Name < fs[j].Name
	})
	return fs
}

func flatten(name string, fs []field) []field {
	var ret []field
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
