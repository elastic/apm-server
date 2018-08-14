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

package beater

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamResponseSimple(t *testing.T) {
	sr := streamResponse{}

	sr.AddError(queueFullErr, 23)

	jsonOut, err := sr.Marshal()
	assert.NoError(t, err)
	expectedJSON := `{
		"accepted":0,
		"invalid":0,
		"dropped":0,
		"errors":{
			"ERR_QUEUE_FULL":{
				"count":23,
				"message":"queue is full"
			}
		}
	}`
	expectedJSON = strings.Replace(strings.Replace(expectedJSON, "\n", "", -1), "\t", "", -1)
	assert.Equal(t, expectedJSON, string(jsonOut))

	expectedStr := `queue is full (23)`
	assert.Equal(t, expectedStr, sr.String())
	assert.Equal(t, 429, sr.StatusCode())

}
func TestStreamResponseAdvanced(t *testing.T) {
	sr := streamResponse{}

	sr.AddError(schemaValidationErr, 1)
	sr.AddError(schemaValidationErr, 4)
	sr.ValidationError("transmogrifier error", `{"wrong": "field"}`)
	sr.ValidationError("transmogrifier error", `{"wrong": "field"}`)
	sr.ValidationError("thing error", `{"wrong": "value"}`)

	sr.AddError(queueFullErr, 23)

	jsonOut, err := sr.Marshal()
	assert.NoError(t, err)
	expectedJSON := `{
		"accepted":0,
		"invalid":0,
		"dropped":0,
		"errors":{
			"ERR_QUEUE_FULL":{
				"count":23,
				"message":"queue is full"
			},
			"ERR_SCHEMA_VALIDATION":{
				"count":5,
				"message":"validation error",
				"documents":[
					{"error":"transmogrifier error","object":"{\"wrong\": \"field\"}"},
					{"error":"thing error","object":"{\"wrong\": \"value\"}"}
				]
			}
		}
	}`
	expectedJSON = strings.Replace(strings.Replace(expectedJSON, "\n", "", -1), "\t", "", -1)
	assert.Equal(t, expectedJSON, string(jsonOut))

	expectedStr := `queue is full (23), validation error (5): transmogrifier error ({"wrong": "field"}), thing error ({"wrong": "value"})`
	assert.Equal(t, expectedStr, sr.String())

	assert.Equal(t, 429, sr.StatusCode())
}
