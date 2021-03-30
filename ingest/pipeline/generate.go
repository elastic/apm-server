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

// +build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type pipelineDefinition struct {
	ID   interface{} `json:"id"`
	Body interface{} `json:"body"`
}

func main() {
	var doc yaml.Node
	fin, err := os.Open("definition.yml")
	if err != nil {
		log.Fatal(err)
	}
	yamlDecoder := yaml.NewDecoder(fin)
	if err := yamlDecoder.Decode(&doc); err != nil {
		log.Fatal(err)
	}

	// Convert the document structure into the one expected by libbeat.
	// e.g. convert {a: 1, b: 2, ...} to [{id: a, body: 1}, {id: b, body: 2}, ...]
	if n := len(doc.Content); n != 1 {
		log.Fatalf("expected 1 document, got %d", n)
	}
	mappingNode := doc.Content[0]
	sequenceNode := &yaml.Node{Kind: yaml.SequenceNode, Content: make([]*yaml.Node, len(mappingNode.Content)/2)}
	for i := 0; i < len(mappingNode.Content); i += 2 {
		idNode := mappingNode.Content[i]
		bodyNode := mappingNode.Content[i+1]
		sequenceNode.Content[i/2] = &yaml.Node{
			Kind: yaml.MappingNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "id"},
				idNode,
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "body"},
				bodyNode,
			},
		}
	}
	doc.Content[0] = sequenceNode

	var buf bytes.Buffer
	if err := encodeJSON(&buf, &doc); err != nil {
		log.Fatal(err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, buf.Bytes(), "", "  "); err != nil {
		log.Fatal(err)
	}
	if err := ioutil.WriteFile("definition.json", indented.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
}

func encodeJSON(buf *bytes.Buffer, node *yaml.Node) error {
	switch node.Kind {
	case yaml.DocumentNode:
		return encodeJSON(buf, node.Content[0])
	case yaml.SequenceNode:
		buf.WriteByte('[')
		for i, node := range node.Content {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encodeJSON(buf, node); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case yaml.MappingNode:
		buf.WriteByte('{')
		for i := 0; i < len(node.Content); i += 2 {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encodeJSON(buf, node.Content[i]); err != nil {
				return err
			}
			buf.WriteByte(':')
			if err := encodeJSON(buf, node.Content[i+1]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
		return nil
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!str":
			enc := json.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			return enc.Encode(node.Value)
		case "!!bool", "!!int":
			buf.WriteString(node.Value)
			return nil
		default:
			return fmt.Errorf("unexpected tag %q at %d:%d", node.Tag, node.Line, node.Column)
		}
	default:
		return fmt.Errorf("unexpected kind %d at %d:%d", node.Kind, node.Line, node.Column)
	}
}
