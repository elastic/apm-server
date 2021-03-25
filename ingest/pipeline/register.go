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

//go:generate go run generate.go

package pipeline

import (
	"encoding/json"
	"io/ioutil"

	logs "github.com/elastic/apm-server/log"

	"github.com/elastic/beats/v7/libbeat/esleg/eslegclient"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/paths"
)

func RegisterPipelines(conn *eslegclient.Connection, overwrite bool, path string) error {
	logger := logp.NewLogger(logs.Pipelines)
	pipelines, err := loadPipelinesFromJSON(path)
	if err != nil {
		return err
	}
	var exists bool
	for _, p := range pipelines {
		if !overwrite {
			exists, err = conn.PipelineExists(p.Id)
			if err != nil {
				return err
			}
		}
		if overwrite || !exists {
			_, _, err := conn.CreatePipeline(p.Id, nil, p.Body)
			if err != nil {
				logger.Errorf("Pipeline registration failed for %s.", p.Id)
				return err
			}
			logger.Infof("Pipeline successfully registered: %s", p.Id)
		} else {
			logger.Infof("Pipeline already registered: %s", p.Id)
		}
	}
	logger.Info("Registered Ingest Pipelines successfully.")
	return nil
}

type pipeline struct {
	Id   string                 `json:"id"`
	Body map[string]interface{} `json:"body"`
}

func loadPipelinesFromJSON(path string) ([]pipeline, error) {
	pipelineDef, err := ioutil.ReadFile(paths.Resolve(paths.Home, path))
	if err != nil {
		return nil, err
	}
	var pipelines []pipeline
	err = json.Unmarshal(pipelineDef, &pipelines)
	return pipelines, err
}
