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

package decoder_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/decoder"
)

func TestDecodeJSONData(t *testing.T) {
	d := decoder.NewJSONDecoder(strings.NewReader(
		`{"id":"85925e55b43f4342","system": {"hostname":"prod1.example.com"},"number":123}`,
	))
	var decoded map[string]interface{}
	err := d.Decode(&decoded)
	assert.Nil(t, err)
	assert.Equal(t, map[string]interface{}{
		"id":     "85925e55b43f4342",
		"system": map[string]interface{}{"hostname": "prod1.example.com"},
		"number": json.Number("123"),
	}, decoded)
}
