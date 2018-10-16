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
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/metadata/generated/schema"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/validation"
)

type MetadataProcessor struct {
	V2TestProcessor
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
		err := validation.Validate(rawMetadata, metadata.ModelSchema())
		if err != nil {
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
		Proc:   &MetadataProcessor{V2TestProcessor{StreamProcessor: stream.StreamProcessor{}}},
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

	metadata, err := decoder.NewNDJSONStreamReader(payloadStream, 100*1024).Read()
	require.NoError(t, err)

	contextMetadata := metadata["metadata"]

	eventFields := tests.NewSet()
	tests.FlattenMapStr(contextMetadata, prefix, nil, eventFields)
	t.Logf("Event fields: %s", eventFields)
	return eventFields
}

func TestMetadataPayloadAttrsMatchFields(t *testing.T) {
	setup := metadataProcSetup()
	eventFields := getMetadataEventAttrs(t, "context")
	allowedNotInFields := tests.NewSet("context.process.argv")
	setup.EventFieldsInTemplateFields(t, eventFields, allowedNotInFields)
}

func TestMetadataPayloadMatchJsonSchema(t *testing.T) {
	metadataProcSetup().AttrsMatchJsonSchema(t,
		getMetadataEventAttrs(t, ""),
		nil,
		nil,
	)
}

func TestKeywordLimitationOnMetadataAttrs(t *testing.T) {
	metadataProcSetup().KeywordLimitation(
		t,
		tests.NewSet("processor.event", "processor.name", "listening",
			tests.Group("context.request"),
			tests.Group("context.tags"),
			tests.Group("transaction"),
			tests.Group("parent"),
			tests.Group("trace"),
		),
		map[string]string{
			"context.": "",
		},
	)
}

func TestInvalidPayloadsForMetadata(t *testing.T) {
	type obj = map[string]interface{}
	type val = []interface{}

	payloadData := []tests.SchemaTestData{
		{Key: "metadata.service.name",
			Valid:   val{"my-service"},
			Invalid: []tests.Invalid{{Msg: "service/properties/name", Values: val{tests.Str1024Special}}},
		}}
	metadataProcSetup().DataValidation(t, payloadData)
}
