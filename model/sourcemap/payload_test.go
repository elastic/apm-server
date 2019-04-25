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

package sourcemap

import (
	"testing"
	"time"

	s "github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"

	"github.com/elastic/beats/libbeat/common"
)

func getStr(data common.MapStr, key string) string {
	rs, _ := data.GetValue(key)
	return rs.(string)
}

func TestDecode(t *testing.T) {
	data, err := loader.LoadData("../testdata/sourcemap/payload.json")
	assert.NoError(t, err)

	sourcemap, err := DecodeSourcemap(data)
	assert.NoError(t, err)

	rs := sourcemap.Transform(&transform.Context{})
	assert.Len(t, rs, 1)
	event := rs[0]
	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
	output := event.Fields["sourcemap"].(common.MapStr)

	assert.Equal(t, "js/bundle.js", getStr(output, "bundle_filepath"))
	assert.Equal(t, "service", getStr(output, "service.name"))
	assert.Equal(t, "1", getStr(output, "service.version"))
	assert.Equal(t, data["sourcemap"], getStr(output, "sourcemap"))

	_, err = DecodeSourcemap(nil)
	require.Error(t, err)
	assert.EqualError(t, err, utility.ErrFetch.Error())
}

func TestTransform(t *testing.T) {
	p := Sourcemap{
		ServiceName:    "myService",
		ServiceVersion: "1.0",
		BundleFilepath: "/my/path",
		Sourcemap:      "mysmap",
	}

	tctx := &transform.Context{}
	events := p.Transform(tctx)
	assert.Len(t, events, 1)
	event := events[0]

	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
	output := event.Fields["sourcemap"].(common.MapStr)

	assert.Equal(t, "/my/path", getStr(output, "bundle_filepath"))
	assert.Equal(t, "myService", getStr(output, "service.name"))
	assert.Equal(t, "1.0", getStr(output, "service.version"))
	assert.Equal(t, "mysmap", getStr(output, "sourcemap"))
}

func TestParseSourcemaps(t *testing.T) {
	fileBytes, err := loader.LoadDataAsBytes("../testdata/sourcemap/bundle.js.map")
	assert.NoError(t, err)
	parser, err := s.Parse("", fileBytes)
	assert.NoError(t, err)

	source, _, _, _, ok := parser.Source(1, 9)
	assert.True(t, ok)
	assert.Equal(t, "webpack:///bundle.js", source)
}

func TestInvalidateCache(t *testing.T) {
	data, err := loader.LoadData("../testdata/sourcemap/payload.json")
	assert.NoError(t, err)

	smapId := sourcemap.Id{Path: "/tmp"}
	smapMapper := smapMapperFake{
		c: map[string]*sourcemap.Mapping{
			"/tmp": &(sourcemap.Mapping{}),
		},
	}
	mapping, err := smapMapper.Apply(smapId, 0, 0)
	require.NoError(t, err)
	assert.NotNil(t, mapping)

	conf := transform.Config{SmapMapper: &smapMapper}
	tctx := &transform.Context{Config: conf}

	sourcemap, err := DecodeSourcemap(data)
	require.NoError(t, err)
	sourcemap.Transform(tctx)

	sourcemap, err = DecodeSourcemap(data)
	require.NoError(t, err)
	sourcemap.Transform(tctx)

	mapping, err = smapMapper.Apply(smapId, 0, 0)
	require.NoError(t, err)
	assert.Nil(t, mapping)
}

type smapMapperFake struct {
	c map[string]*sourcemap.Mapping
}

func (a *smapMapperFake) Apply(id sourcemap.Id, lineno, colno int) (*sourcemap.Mapping, error) {
	return a.c[id.Path], nil
}

func (sm *smapMapperFake) NewSourcemapAdded(id sourcemap.Id) {
	sm.c = map[string]*sourcemap.Mapping{}
}
