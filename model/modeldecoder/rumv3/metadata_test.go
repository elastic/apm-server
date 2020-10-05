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

package rumv3

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestMetadataResetModelOnRelease(t *testing.T) {
	inp := `{"m":{"se":{"n":"service-a"}}}`
	m := fetchMetadataRoot()
	require.NoError(t, decoder.NewJSONIteratorDecoder(strings.NewReader(inp)).Decode(m))
	require.True(t, m.IsSet())
	releaseMetadataRoot(m)
	assert.False(t, m.IsSet())
}

func TestDecodeNestedMetadata(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		var out model.Metadata
		testMinValidMetadata := `{"m":{"se":{"n":"name","a":{"n":"go","ve":"1.0.0"}}}}`
		dec := decoder.NewJSONIteratorDecoder(strings.NewReader(testMinValidMetadata))
		require.NoError(t, DecodeNestedMetadata(dec, &out))
		assert.Equal(t, model.Metadata{Service: model.Service{
			Name:  "name",
			Agent: model.Agent{Name: "go", Version: "1.0.0"}}}, out)

		err := DecodeNestedMetadata(decoder.NewJSONIteratorDecoder(strings.NewReader(`malformed`)), &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		inp := `{}`
		var out model.Metadata
		err := DecodeNestedMetadata(decoder.NewJSONIteratorDecoder(strings.NewReader(inp)), &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})

}

func TestDecodeMetadataMappingToModel(t *testing.T) {
	expected := func(s string) model.Metadata {
		return model.Metadata{
			Service: model.Service{Name: s, Version: s, Environment: s,
				Agent:     model.Agent{Name: s, Version: s},
				Language:  model.Language{Name: s, Version: s},
				Runtime:   model.Runtime{Name: s, Version: s},
				Framework: model.Framework{Name: s, Version: s}},
			User:   model.User{Name: s, Email: s, ID: s},
			Labels: common.MapStr{s: s},
		}
	}

	// setup:
	// create initialized modeldecoder and empty model metadata
	// map modeldecoder to model metadata and manually set
	// enhanced data that are never set by the modeldecoder
	var m metadata
	modeldecodertest.SetStructValues(&m, "init", 5000, false, time.Now())
	var modelM model.Metadata
	mapToMetadataModel(&m, &modelM)
	// iterate through model and assert values are set
	assert.Equal(t, expected("init"), modelM)

	// overwrite model metadata with specified Values
	// then iterate through model and assert values are overwritten
	modeldecodertest.SetStructValues(&m, "overwritten", 12, false, time.Now())
	mapToMetadataModel(&m, &modelM)
	assert.Equal(t, expected("overwritten"), modelM)

	// map an empty modeldecoder metadata to the model
	// and assert values are unchanged
	modeldecodertest.SetZeroStructValues(&m)
	mapToMetadataModel(&m, &modelM)
	assert.Equal(t, expected("overwritten"), modelM)

}
