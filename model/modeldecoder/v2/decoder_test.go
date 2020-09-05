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

package v2

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
)

func TestResetModelOnRelease(t *testing.T) {
	inp := `{"metadata":{"service":{"name":"service-a"}}}`
	m := fetchMetadataWithKey()
	require.NoError(t, decoder.NewJSONIteratorDecoder(strings.NewReader(inp)).Decode(m))
	require.True(t, m.IsSet())
	releaseMetadataWithKey(m)
	assert.False(t, m.IsSet())

	mNoKey := fetchMetadataNoKey()
	require.NoError(t, decoder.NewJSONIteratorDecoder(strings.NewReader(inp)).Decode(mNoKey))
	require.True(t, mNoKey.IsSet())
	releaseMetadataNoKey(mNoKey)
	assert.False(t, mNoKey.IsSet())
}

func TestDecodeProfileMetadata(t *testing.T) {
	var testMinValidMetadata = `
	{"service":{"name":"user-service","agent":{"name":"go","version":"1.0.0"}}}`

	decodeProfileMetadata := func(out *model.Metadata) {
		dec := decoder.NewJSONIteratorDecoder(strings.NewReader(testMinValidMetadata))
		require.NoError(t, DecodeProfileMetadata(dec, out))
	}

	t.Run("fetch-release", func(t *testing.T) {
		// this test cannot be run in parallel with other tests using the
		// metadataNoKeyPool sync.Pool

		// whenever DecodeProfileMetadata is called a metadata instance
		// should be retrieved from the sync.Pool and be released on finish
		// test behavior by overriding the new method, occupying an existing
		// instance and counting how often the New method is called
		var newCount, expectedNewCount int
		origNew := metadataNoKeyPool.New
		defer func() { metadataNoKeyPool.New = origNew }()
		metadataNoKeyPool.New = func() interface{} {
			newCount++
			return &metadataNoKey{}
		}
		var out model.Metadata
		// on the first call align the expected with the current new count
		// important since other tests might have run already
		decodeProfileMetadata(&out)
		expectedNewCount = newCount
		// on the second call it should reuse the metadata instance
		decodeProfileMetadata(&out)
		assert.Equal(t, expectedNewCount, newCount)
		// force a new instance on the next decoder call
		fetchMetadataNoKey()
		decodeProfileMetadata(&out)
		assert.Equal(t, expectedNewCount+1, newCount)
	})

	t.Run("decode", func(t *testing.T) {
		var out model.Metadata
		decodeProfileMetadata(&out)
		assert.Equal(t, model.Metadata{Service: model.Service{
			Name:  "user-service",
			Agent: model.Agent{Name: "go", Version: "1.0.0"}}}, out)

		err := DecodeProfileMetadata(decoder.NewJSONIteratorDecoder(strings.NewReader(`malformed`)), &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		inp := `{}`
		var out model.Metadata
		err := DecodeProfileMetadata(decoder.NewJSONIteratorDecoder(strings.NewReader(inp)), &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})

}

func TestDecodeMetadata(t *testing.T) {
	var testMinValidMetadata = `
	{"metadata":{"service":{"name":"user-service","agent":{"name":"go","version":"1.0.0"}}}}`

	decodeMetadata := func(out *model.Metadata) {
		dec := decoder.NewJSONIteratorDecoder(strings.NewReader(testMinValidMetadata))
		require.NoError(t, DecodeMetadata(dec, out))
	}

	t.Run("fetch-release", func(t *testing.T) {
		// this test cannot be run in parallel with other tests using the
		// metadataNoKeyPool sync.Pool

		// whenever DecodeProfileMetadata is called a metadata instance
		// should be retrieved from the sync.Pool and be released on finish
		// test behavior by overriding the new method, occupying an existing
		// instance and counting how often the New method is called
		var newCount, expectedNewCount int
		origNew := metadataWithKeyPool.New
		defer func() { metadataWithKeyPool.New = origNew }()
		metadataWithKeyPool.New = func() interface{} {
			newCount++
			return &metadataWithKey{}
		}
		var out model.Metadata
		// on the first call align the expected with the current new count
		// important since other tests might have run already
		decodeMetadata(&out)
		expectedNewCount = newCount
		// on the second call it should reuse the metadata instance
		decodeMetadata(&out)
		assert.Equal(t, expectedNewCount, newCount)
		// force a new instance on the next decoder call
		fetchMetadataWithKey()
		decodeMetadata(&out)
		assert.Equal(t, expectedNewCount+1, newCount)
	})

	t.Run("decode", func(t *testing.T) {
		var out model.Metadata
		decodeMetadata(&out)
		assert.Equal(t, model.Metadata{Service: model.Service{
			Name:  "user-service",
			Agent: model.Agent{Name: "go", Version: "1.0.0"}}}, out)

		err := DecodeMetadata(decoder.NewJSONIteratorDecoder(strings.NewReader(`malformed`)), &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		inp := `{}`
		var out model.Metadata
		err := DecodeMetadata(decoder.NewJSONIteratorDecoder(strings.NewReader(inp)), &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})

}

func TestMappingToModel(t *testing.T) {
	//TODO(simitt)
	//  override existing values if they are set
}
