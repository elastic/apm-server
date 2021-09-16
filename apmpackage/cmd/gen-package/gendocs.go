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
	"strings"
	"text/template"
)

func escapeReplacer(s ...string) *strings.Replacer {
	pairs := make([]string, len(s)*2)
	for i, s := range s {
		pairs[2*i] = s
		pairs[2*i+1] = "\\" + s
	}
	return strings.NewReplacer(pairs...)
}

var markdownReplacer = escapeReplacer("\\", "`", "*", "_")

func generateDocs(inputFields map[string]fieldMap) {
	addBaseFields(inputFields, "traces", "app_metrics", "error_logs")
	data := docsData{
		Traces:             flattenFields(inputFields["traces"]),
		Metrics:            flattenFields(inputFields["app_metrics"]),
		Logs:               flattenFields(inputFields["error_logs"]),
		TransactionExample: loadExample("generated/transactions.json"),
		SpanExample:        loadExample("generated/spans.json"),
		MetricsExample:     loadExample("metricset.json"),
		ErrorExample:       loadExample("generated/errors.json"),
	}
	t := template.New(docsTemplateFilePath())
	tmpl, err := t.Funcs(map[string]interface{}{
		"Trim":           strings.TrimSpace,
		"EscapeMarkdown": markdownReplacer.Replace,
	}).ParseFiles(docsTemplateFilePath())
	if err != nil {
		panic(err)
	}
	path := docsFilePath()
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

func addBaseFields(streamFields map[string]fieldMap, streams ...string) {
	for _, stream := range streams {
		fields := streamFields[stream]
		for _, f := range loadFieldsFile(baseFieldsFilePath(stream)) {
			f.IsECS = true
			fields[f.Name] = fieldMapItem{field: f}
		}
	}
}

func loadExample(file string) string {
	in, err := ioutil.ReadFile(path.Join("docs/data/elasticsearch/", file))
	if err != nil {
		panic(err)
	}
	var aux interface{}
	err = json.Unmarshal(in, &aux)
	if err != nil {
		panic(err)
	}
	if slice, ok := aux.([]interface{}); ok {
		aux = slice[0]
	}
	out, err := json.MarshalIndent(aux, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(out)
}
