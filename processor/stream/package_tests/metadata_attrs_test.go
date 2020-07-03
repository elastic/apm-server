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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata/generated/schema"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
)

type MetadataProcessor struct {
	intakeTestProcessor
}

func (p *MetadataProcessor) LoadPayload(path string) (interface{}, error) {
	ndjson, err := p.getReader(path)
	if err != nil {
		return nil, err
	}

	return p.readEvents(ndjson)
}

func (p *MetadataProcessor) Validate(data interface{}) error {
	events := data.([]interface{})
	for _, e := range events {
		rawEvent := e.(map[string]interface{})
		rawMetadata, ok := rawEvent["metadata"].(map[string]interface{})
		if !ok {
			return stream.ErrUnrecognizedObject
		}

		// validate the metadata object against our jsonschema
		if err := modeldecoder.DecodeMetadata(rawMetadata, false, &model.Metadata{}); err != nil {
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
		Schema: schema.ModelSchema,
		TemplatePaths: []string{
			"../../../_meta/fields.common.yml",
		},
		FullPayloadPath: "../testdata/intake-v2/only-metadata.ndjson",
	}
}

func getMetadataEventAttrs(t *testing.T, prefix string) *tests.Set {
	payloadStream, err := loader.LoadDataAsStream("../testdata/intake-v2/only-metadata.ndjson")
	require.NoError(t, err)

	metadata, err := decoder.NewNDJSONStreamReader(payloadStream, lrSize).Read()
	require.NoError(t, err)

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

func TestMetadataPayloadMatchJsonSchema(t *testing.T) {
	metadataProcSetup().AttrsMatchJsonSchema(t,
		getMetadataEventAttrs(t, ""),
		tests.NewSet(tests.Group("labels")),
		nil,
	)
}

func TestKeywordLimitationOnMetadataAttrs(t *testing.T) {
	metadataProcSetup().KeywordLimitation(
		t,
		tests.NewSet("processor.event", "processor.name",
			"process.args",
			tests.Group("observer"),
			tests.Group("http"),
			tests.Group("url"),
			tests.Group("context.tags"),
			tests.Group("transaction"),
			tests.Group("span"),
			tests.Group("parent"),
			tests.Group("trace"),
			tests.Group("user_agent"),
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

func TestInvalidPayloadsForMetadata(t *testing.T) {
	type val []interface{}

	payloadData := []tests.SchemaTestData{
		{Key: "metadata.service.name",
			Valid:   val{"my-service"},
			Invalid: []tests.Invalid{{Msg: "service/properties/name", Values: val{tests.Str1024Special}}},
		}}
	metadataProcSetup().DataValidation(t, payloadData)
}
