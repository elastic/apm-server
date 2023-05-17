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
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-agent-libs/version"
)

const (
	integrationName = "apm"
)

func transformFile(path string, content []byte, version, ecsVersion *version.V, ecsReference, interval string) ([]byte, error) {
	if path == "manifest.yml" {
		return transformPackageManifest(content, version)
	}
	if isDataStreamManifest(path) {
		return transformDataStreamManifest(path, content, version, interval)
	}
	if isIngestPipeline(path) {
		return transformIngestPipeline(path, content, version)
	}
	if isECSFieldsYAML(path) {
		return transformECSFieldsYAML(content, ecsVersion)
	}
	if path == "changelog.yml" {
		return transformChangelog(content, version)
	}
	if path == "_dev/build/build.yml" {
		return transformBuildDependencies(content, ecsReference)
	}
	return content, nil
}

func transformECSFieldsYAML(content []byte, ecsVersion *version.V) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, err
	}

	var found bool
	for _, fieldNode := range doc.Content[0].Content {
		if yamlMapLookup(fieldNode, "name").Value != "ecs.version" {
			continue
		}
		fieldNode.Content = append(fieldNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "type"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "constant_keyword"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "value"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: ecsVersion.String()},
		)
		found = true
		break
	}
	if !found {
		return content, nil
	}

	return marshalYAML(&doc)
}

func transformBuildDependencies(content []byte, ecsReference string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, err
	}

	// Set the ECS version.
	node := yamlMapLookup(doc.Content[0], "dependencies", "ecs", "reference")
	node.Value = ecsReference

	return marshalYAML(&doc)
}

func transformPackageManifest(content []byte, version *version.V) ([]byte, error) {
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

func transformDataStreamManifest(path string, content []byte, version *version.V, interval string) ([]byte, error) {
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
	if interval != "" {
		dataStreamName = strings.Replace(dataStreamName, "_interval_", fmt.Sprintf("_%s_", interval), -1)
		expected = fmt.Sprintf("%s-%s.%s-default_policy", dataStreamType, integrationName, dataStreamName)
	}
	if ilmPolicy.Value != expected {
		return nil, fmt.Errorf("expected ilm_policy to be %q, got %q", expected, ilmPolicy.Value)
	}

	return content, nil
}

func transformIngestPipeline(path string, content []byte, version *version.V) ([]byte, error) {
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

func transformChangelog(content []byte, version *version.V) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, err
	}

	n := doc.Content[0]
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("expected list of versions (%v), got %v", yaml.SequenceNode, n.Kind)
	}

	for i := 0; i < len(n.Content); i++ {
		changelogEntryVersion := yamlMapLookup(n.Content[i], "version")
		if changelogEntryVersion.Value == "generated" {
			changelogEntryVersion.Value = version.String()
		}
	}
	return marshalYAML(&doc)
}

// resolveIngestPipelineReferences resolves pipeline processors to common
// pipelines defined in common_pipelines.yml, which holds a map of pipeline
// names to sequences of processors.
func resolveIngestPipelineReferences(doc *yaml.Node, version *version.V) error {
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

func isECSFieldsYAML(path string) bool {
	dir, file := filepath.Split(path)
	if file != "ecs.yml" {
		return false
	}
	return filepath.Base(dir) == "fields"
}

func yamlMapLookup(n *yaml.Node, key ...string) *yaml.Node {
	if n.Kind != yaml.MappingNode {
		panic(fmt.Sprintf("expected node kind %v, got %v", yaml.MappingNode, n.Kind))
	}
	for _, key := range key {
		var found bool
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind == yaml.ScalarNode && k.Value == key {
				n = n.Content[i+1]
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	return n
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
