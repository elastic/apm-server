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
	"fmt"
	"testing"
	"time"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/publish"

	"github.com/elastic/apm-server/model/sourcemap/generated/schema"
	"github.com/elastic/apm-server/tests/approvals"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/beats/libbeat/beat"

	"github.com/elastic/apm-server/processor/asset/sourcemap"
	"github.com/elastic/apm-server/tests"
)

type sourcemapEventDecoder struct{}

func (s sourcemapEventDecoder) Decode(input interface{}, _ time.Time, _ metadata.Metadata) (publish.Transformable, error) {
	if err := sourcemap.Processor.Validate(input.(map[string]interface{})); err != nil {
		return nil, err
	}
	transformables, err := sourcemap.Processor.Decode(input.(map[string]interface{}), nil)
	if err != nil {
		return nil, err
	}
	return transformables[0], err
}

func sourcemapProcSetup() *tests.ProcessorSetup {
	path := "../testdata/sourcemap/payload.json"
	payload, _ := loader.LoadData(path)
	return &tests.ProcessorSetup{
		Decoder:         sourcemapEventDecoder{},
		FullPayloadPath: path,
		SamplePayload:   payload,
		TemplatePaths:   []string{"../../../../model/sourcemap/_meta/fields.yml"},
		Schema:          schema.PayloadSchema,
	}
}

// ensure all valid documents pass through the whole validation and transformation process
func TestSourcemapProcessorOK(t *testing.T) {
	data := []struct {
		Name string
		Path string
	}{
		{Name: "TestProcessSourcemapFull", Path: "../testdata/sourcemap/payload.json"},
		{Name: "TestProcessSourcemapMinimalPayload", Path: "../testdata/sourcemap/minimal_payload.json"},
	}

	for _, info := range data {
		p := sourcemap.Processor

		data, err := loader.LoadData(info.Path)
		require.NoError(t, err)

		err = p.Validate(data)
		require.NoError(t, err)

		payload, err := p.Decode(data, nil)
		require.NoError(t, err)

		var events []beat.Event
		for _, transformable := range payload {
			events = append(events, transformable.Transform()...)
		}
		verifyErr := approvals.ApproveEvents(events, info.Name, "@timestamp")
		if verifyErr != nil {
			assert.Fail(t, fmt.Sprintf("Test %s failed with error: %s", info.Name, verifyErr.Error()))
		}
	}
}

func TestPayloadAttrsMatchFields(t *testing.T) {
	sourcemapProcSetup().PayloadAttrsMatchFields(t, tests.NewSet("sourcemap.sourcemap"), tests.NewSet(), true)
}

func TestPayloadAttrsMatchJsonSchema(t *testing.T) {
	proc := sourcemapProcSetup()
	payload, err := loader.LoadData(proc.FullPayloadPath)
	require.NoError(t, err)
	proc.PayloadAttrsMatchJSONSchema(t,
		tests.NewSet("sourcemap", "sourcemap.file", "sourcemap.names",
			"sourcemap.sources", "sourcemap.sourceRoot"), tests.NewSet(), payload)
}

func TestAttributesPresenceRequirementInSourcemap(t *testing.T) {
	sourcemapProcSetup().AttrsPresence(t,
		tests.NewSet("service_name", "service_version",
			"bundle_filepath", "sourcemap"), nil)
}

func TestKeywordLimitationOnSourcemapAttributes(t *testing.T) {
	mapping := []tests.FieldTemplateMapping{
		{Template: "sourcemap.service.name", Mapping: "service_name"},
		{Template: "sourcemap.service.version", Mapping: "service_version"},
		{Template: "sourcemap.bundle_filepath", Mapping: "bundle_filepath"},
	}

	sourcemapProcSetup().KeywordLimitation(t, tests.NewSet(), mapping)
}

func TestPayloadDataForSourcemap(t *testing.T) {
	type val []interface{}
	payloadData := []tests.SchemaTestData{
		// add test data for testing
		// * specific edge cases
		// * multiple allowed dataypes
		// * regex pattern, time formats
		// * length restrictions, other than keyword length restrictions

		{Key: "sourcemap", Invalid: []tests.Invalid{
			{Msg: `error validating sourcemap`, Values: val{""}},
			{Msg: `sourcemap not in expected format`, Values: val{[]byte{}}}}},
		{Key: "service_name", Valid: val{tests.Str1024},
			Invalid: []tests.Invalid{
				{Msg: `service_name/minlength`, Values: val{""}},
				{Msg: `service_name/maxlength`, Values: val{tests.Str1025}},
				{Msg: `service_name/pattern`, Values: val{tests.Str1024Special}}}},
		{Key: "service_version", Valid: val{tests.Str1024},
			Invalid: []tests.Invalid{{Msg: `service_version/minlength`, Values: val{""}}}},
		{Key: "bundle_filepath", Valid: []interface{}{tests.Str1024},
			Invalid: []tests.Invalid{{Msg: `bundle_filepath/minlength`, Values: val{""}}}},
	}
	sourcemapProcSetup().DataValidation(t, payloadData)
}
