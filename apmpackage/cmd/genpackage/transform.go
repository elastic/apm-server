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
	"bytes"
	"errors"
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/elastic/beats/v7/libbeat/common"
)

const (
	integrationName = "apm"
)

func transformFile(path string, content []byte, version *common.Version) ([]byte, error) {
	if path == "manifest.yml" {
		return transformPackageManifest(content, version)
	}
	if isDataStreamManifest(path) {
		return transformDataStreamManifest(path, content, version)
	}
	if isIngestPipeline(path) {
		return transformIngestPipeline(path, content, version)
	}
	return content, nil
}

func transformPackageManifest(content []byte, version *common.Version) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, err
	}

	// Set the integration package version to the exact version, and the minimum
	// Kibana version to Major.Minor.0.
	yamlMapLookup(doc.Content[0], "version").Value = version.String()

	// Set the minimum Kibana version to major.minor.0.
	conditions := yamlMapLookup(doc.Content[0], "conditions")
	yamlMapLookup(conditions, "kibana.version").Value = fmt.Sprintf("^%d.%d.0", version.Major, version.Minor)

	return marshalYAML(&doc)
}

func transformDataStreamManifest(path string, content []byte, version *common.Version) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, err
	}

	// Make sure the ilm_policy field is correct. We should always have a
	// default data stream-specific policy and its name should be
	// "<type>-<integration>.<datastream_directory>-default_policy".
	ilmPolicy := yamlMapLookup(doc.Content[0], "ilm_policy")
	if ilmPolicy == nil {
		return nil, errors.New("ilm_policy not defined")
	}
	dataStreamType := yamlMapLookup(doc.Content[0], "type").Value
	dataStreamName := filepath.Base(filepath.Dir(path))
	expected := fmt.Sprintf("%s-%s.%s-default_policy", dataStreamType, integrationName, dataStreamName)
	if ilmPolicy.Value != expected {
		return nil, fmt.Errorf("expected ilm_policy to be %q, got %q", expected, ilmPolicy.Value)
	}

	return content, nil
}

func transformIngestPipeline(path string, content []byte, version *common.Version) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, err
	}
	if err := resolveIngestPipelineReferences(&doc, version); err != nil {
		return nil, err
	}
	content, err := marshalYAML(&doc)
	if err != nil {
		return nil, err
	}
	// Elasticsearch expects YAML ingest pipeline definitions to begin with "---".
	return append([]byte("---\n"), content...), nil
}

// resolveIngestPipelineReferences resolves pipeline processors to common
// pipelines defined in common_pipelines.yml, which holds a map of pipeline
// names to sequences of processors.
func resolveIngestPipelineReferences(doc *yaml.Node, version *common.Version) error {
	processorsNode := yamlMapLookup(doc.Content[0], "processors")
	for i := 0; i < len(processorsNode.Content); i++ {
		node := processorsNode.Content[i]
		pipelineNode := yamlMapLookup(node, "pipeline")
		if pipelineNode == nil {
			continue
		}
		pipelineNameNode := yamlMapLookup(pipelineNode, "name")
		if pipelineNameNode == nil {
			continue
		}

		pipeline := getCommonPipeline(pipelineNameNode.Value, version)
		if pipeline == nil {
			return fmt.Errorf("pipeline %q not found", pipelineNameNode.Value)
		}
		var replacement yaml.Node
		if err := replacement.Encode(pipeline); err != nil {
			return err
		}

		processorsNode.Content = append(
			processorsNode.Content[:i],
			append(replacement.Content[:], processorsNode.Content[i+1:]...)...,
		)
		i += len(replacement.Content) - 1
	}
	return nil
}

func isIngestPipeline(path string) bool {
	dir := filepath.Dir(path)
	return filepath.Base(dir) == "ingest_pipeline"
}

func isDataStreamManifest(path string) bool {
	if filepath.Base(path) != "manifest.yml" {
		return false
	}
	dir := filepath.Dir(path) // .../data_stream/<foo>
	dir = filepath.Dir(dir)   // .../data_stream
	return filepath.Base(dir) == "data_stream"
}

func yamlMapLookup(n *yaml.Node, key string) *yaml.Node {
	if n.Kind != yaml.MappingNode {
		panic(fmt.Sprintf("expected node kind %v, got %v", yaml.MappingNode, n.Kind))
	}
	for i := 0; i < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func marshalYAML(doc *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
