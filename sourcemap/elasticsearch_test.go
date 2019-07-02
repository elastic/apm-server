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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
	es "github.com/elastic/beats/libbeat/outputs/elasticsearch"
)

func TestNewElasticsearch(t *testing.T) {
	_, err := NewElasticsearch(getFakeESConfig(map[string]interface{}{}), "apm")
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, InitError)
	assert.Contains(t, err.Error(), "ES Client cannot be initialized")

	es, err := NewElasticsearch(getFakeESConfig(nil), "")
	assert.NoError(t, err)
	assert.Equal(t, "*", es.index)
}

func TestNoElasticsearchConnection(t *testing.T) {
	es, err := NewElasticsearch(getFakeESConfig(nil), "")
	assert.NoError(t, err)
	c, err := es.fetch(Id{ServiceName: "testService"})
	assert.Nil(t, c)
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, AccessError)
	assert.Contains(t, err.Error(), "connection")
}

func TestParseResultNoSourcemap(t *testing.T) {
	id := Id{Path: "/tmp"}
	result := &es.SearchResults{}
	c, err := (&smapElasticsearch{}).parseResult(result, id)
	assert.Nil(t, c)
	assert.NoError(t, err)
}

func TestParseResultParseError(t *testing.T) {
	id := Id{Path: "/tmp"}
	result := &es.SearchResults{
		Hits: es.Hits{
			Hits: []json.RawMessage{
				{},
			},
			Total: es.Total{Value: 1},
		},
	}
	c, err := (&smapElasticsearch{}).parseResult(result, id)
	assert.Nil(t, c)
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, ParseError)

	result = &es.SearchResults{
		Hits: es.Hits{
			Hits: []json.RawMessage{
				[]byte(`{"_id": "1","_source": {"sourcemap": {"sourcemap": "map"}}}`),
			},
			Total: es.Total{Value: 1},
		},
	}
	c, err = (&smapElasticsearch{}).parseResult(result, id)
	assert.Nil(t, c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Could not parse Sourcemap")
	assert.Equal(t, (err.(Error)).Kind, ParseError)
}

func TestParseSourcemapResult(t *testing.T) {
	smap, err := parseSmap([]byte(`{
"_id": "1",
"_source": {
"sourcemap": {
"sourcemap": "map"
}
}
}
`))
	assert.NoError(t, err)
	assert.Equal(t, "map", smap)
}

func TestParseSourcemapResultError(t *testing.T) {
	// valid json, missing sourcemap
	_, err := parseSmap([]byte(`{
"_id": "1",
"_source": {
"foo": "bar"
}
}
`))
	assert.Error(t, err)

	// invalid json
	_, err = parseSmap([]byte(`{`))
	assert.Error(t, err)
}

func getFakeESConfig(cfg map[string]interface{}) *common.Config {
	if cfg == nil {
		cfg = map[string]interface{}{
			"hosts": []string{
				"http://localhost:9288",
				"http://localhost:9898",
			},
		}
	}
	c, _ := common.NewConfigFrom(cfg)
	return c
}
