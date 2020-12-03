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
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
)

var streamMappings = map[string]string{
	"logs":             "logs-apm.error",
	"traces":           "traces-apm",
	"metrics":          "metrics-apm",
	"internal_metrics": "metrics-apm.internal",
	"profiles":         "profiles-apm",
}

type PipelineDef struct {
	ID   string       `json:"id"`
	Body PipelineBody `json:"body"`
}

type PipelineBody struct {
	Description string      `json:"description"`
	Processors  []Processor `json:"processors"`
}

type Processor struct {
	Pipeline *Pipeline `json:"pipeline,omitempty"`
	m        map[string]interface{}
}

type Pipeline struct {
	Name string `json:"name"`
}

type _Processor Processor

func (p *Processor) UnmarshalJSON(bytes []byte) error {
	aux := _Processor{}
	err := json.Unmarshal(bytes, &aux)
	if err != nil {
		return err
	}

	*p = Processor(aux)
	m := make(map[string]interface{})

	err = json.Unmarshal(bytes, &m)
	if err != nil {
		return err
	}
	delete(m, "pipeline")
	p.m = m
	return nil
}

func (p *Processor) MarshalJSON() ([]byte, error) {
	aux := _Processor(*p)
	if p.Pipeline != nil {
		return json.Marshal(aux)
	}
	return json.Marshal(p.m)
}

func generatePipelines(version, dataStream string) {
	pipelines, err := os.Open("ingest/pipeline/definition.json")
	if err != nil {
		panic(err)
	}
	defer pipelines.Close()

	bytes, err := ioutil.ReadAll(pipelines)
	if err != nil {
		panic(err)
	}

	var definitions = make([]PipelineDef, 0)
	err = json.Unmarshal(bytes, &definitions)
	if err != nil {
		panic(err)
	}

	os.MkdirAll(pipelinesPath(version, dataStream), 0755)

	var apmPipeline PipelineBody
	for _, definition := range definitions {
		pipeline := definition.Body
		if definition.ID == "apm" {
			apmPipeline = pipeline
			continue
		}
		out, err := json.MarshalIndent(pipeline, "", "  ")
		if err != nil {
			panic(err)
		}
		fName := filepath.Join(pipelinesPath(version, dataStream), definition.ID+".json")
		ioutil.WriteFile(fName, out, 0644)
	}

	for _, p := range apmPipeline.Processors {
		if p.Pipeline == nil {
			// should not happen, lets panic loudly
			panic(errors.New("expected pipeline processor"))
		}
		p.Pipeline.Name = streamMappings[dataStream] + "-" + version + "-" + p.Pipeline.Name
	}
	out, err := json.MarshalIndent(apmPipeline, "", "  ")
	if err != nil {
		panic(err)
	}
	fName := filepath.Join(pipelinesPath(version, dataStream), "default.json")
	ioutil.WriteFile(fName, out, 0644)
}
