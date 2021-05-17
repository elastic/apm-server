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

package package_tests

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	v2 "github.com/elastic/apm-server/model/modeldecoder/v2"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
)

type MetadataProcessor struct {
	intakeTestProcessor
}

func (p *MetadataProcessor) LoadPayload(path string) (interface{}, error) {
	ndjson, err := p.getDecoder(path)
	if err != nil {
		return nil, err
	}

	return p.readEvents(ndjson)
}

func (p *MetadataProcessor) Validate(data interface{}) error {
	events := data.([]interface{})
	for _, e := range events {
		//TODO(simitt): combine loading the data and validating them once the new json decoding is finished
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		dec := decoder.NewJSONDecoder(bytes.NewReader(b))
		var m model.Metadata
		if err := v2.DecodeNestedMetadata(dec, &m); err != nil {
			return err
		}
	}

	return nil
}

func (p *MetadataProcessor) Decode(data interface{}) error {
	return p.Validate(data)
}

func metadataProcSetup() *tests.ProcessorSetup {
	return &tests.ProcessorSetup{
		Proc: &MetadataProcessor{
			intakeTestProcessor{Processor: stream.Processor{MaxEventSize: lrSize}}},
		SchemaPath: "../../../docs/spec/v2/metadata.json",
		TemplatePaths: []string{
			// we use the fields.yml file of a type that includes all the metadata fields
			// this was changed with the removal of fields.common.yml
			// TODO: move metadata package tests into event specific tests when refactoring package tests
			"../../../model/transaction/_meta/fields.yml",
		},
		FullPayloadPath: "../testdata/intake-v2/metadata.ndjson",
	}
}

func getMetadataEventAttrs(t *testing.T, prefix string) *tests.Set {
	payloadStream, err := loader.LoadDataAsStream("../testdata/intake-v2/metadata.ndjson")
	require.NoError(t, err)

	var metadata map[string]interface{}
	require.NoError(t, decoder.NewNDJSONStreamDecoder(payloadStream, lrSize).Decode(&metadata))

	contextMetadata := metadata["metadata"]

	eventFields := tests.NewSet()
	tests.FlattenMapStr(contextMetadata, prefix, nil, eventFields)
	t.Logf("Event field: %s", eventFields)
	return eventFields
}

func TestMetadataPayloadAttrsMatchFields(t *testing.T) {
	setup := metadataProcSetup()
	eventFields := getMetadataEventAttrs(t, "")

	var mappingFields = []tests.FieldTemplateMapping{
		{Template: "system.container.", Mapping: "container."},             // move system.container.*
		{Template: "system.container", Mapping: ""},                        // delete system.container
		{Template: "system.kubernetes.node.", Mapping: "kubernetes.node."}, // move system.kubernetes.node.*
		{Template: "system.kubernetes.node", Mapping: ""},                  // delete system.kubernetes.node
		{Template: "system.kubernetes.pod.", Mapping: "kubernetes.pod."},   // move system.kubernetes.pod.*
		{Template: "system.kubernetes.pod", Mapping: ""},                   // delete system.kubernetes.pod
		{Template: "system.kubernetes.", Mapping: "kubernetes."},           // move system.kubernetes.*
		{Template: "system.kubernetes", Mapping: ""},                       // delete system.kubernetes
		{Template: "system.platform", Mapping: "host.os.platform"},
		{Template: "system.configured_hostname", Mapping: "host.name"},
		{Template: "system.detected_hostname", Mapping: "host.hostname"},
		{Template: "system", Mapping: "host"},
		{Template: "service.agent", Mapping: "agent"},
		{Template: "user.username", Mapping: "user.name"},
		{Template: "process.argv", Mapping: "process.args"},
		{Template: "labels.*", Mapping: "labels"},
		{Template: "service.node.configured_name", Mapping: "service.node.name"},
		{Template: "cloud", Mapping: "cloud"},
	}
	setup.EventFieldsMappedToTemplateFields(t, eventFields, mappingFields)
}

func TestKeywordLimitationOnMetadataAttrs(t *testing.T) {
	metadataProcSetup().KeywordLimitation(
		t,
		tests.NewSet(
			"data_stream.type", "data_stream.dataset", "data_stream.namespace",
			"processor.event", "processor.name",
			"process.args",
			tests.Group("observer"),
			tests.Group("event"),
			tests.Group("http"),
			tests.Group("url"),
			tests.Group("context.tags"),
			tests.Group("transaction"),
			tests.Group("session"),
			tests.Group("span"),
			tests.Group("parent"),
			tests.Group("trace"),
			tests.Group("user_agent"),
			tests.Group("client"),
			tests.Group("source"),
			tests.Group("destination"),
		),
		[]tests.FieldTemplateMapping{
			{Template: "agent.", Mapping: "service.agent."},
			{Template: "container.", Mapping: "system.container."},
			{Template: "kubernetes.", Mapping: "system.kubernetes."},
			{Template: "host.os.platform", Mapping: "system.platform"},
			{Template: "host.name", Mapping: "system.configured_hostname"},
			{Template: "host.", Mapping: "system."},
			{Template: "user.name", Mapping: "user.username"},
			{Template: "service.node.name", Mapping: "service.node.configured_name"},
			//{Template: "url.", Mapping:"context.request.url."},
		},
	)
}
