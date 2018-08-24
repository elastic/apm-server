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

package stream

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/tests"
)

func TestStreamResponseSimple(t *testing.T) {
	sr := Result{}
	sr.Add(QueueFullErr, 23)

	jsonByte, err := sr.Marshal()
	require.NoError(t, err)

	var jsonOut map[string]interface{}
	err = json.Unmarshal(jsonByte, &jsonOut)
	require.NoError(t, err)

	verifyErr := tests.ApproveJson(jsonOut, "approved-stream-response/testStreamResponseSimple", nil)
	if verifyErr != nil {
		assert.Fail(t, fmt.Sprintf("Test %s failed with error: %s", "testStreamResponseSimple", verifyErr.Error()))
	}

	expectedStr := `queue is full (23)`
	assert.Equal(t, expectedStr, sr.String())
	assert.Equal(t, 429, sr.StatusCode())
}

func TestStreamResponseAdvanced(t *testing.T) {
	sr := Result{}

	sr.Add(SchemaValidationErr, 2)
	sr.AddWithOffendingDocument(SchemaValidationErr, "transmogrifier error", []byte(`{"wrong": "field"}`))
	sr.AddWithOffendingDocument(SchemaValidationErr, "transmogrifier error", []byte(`{"wrong": "field"}`))
	sr.AddWithOffendingDocument(SchemaValidationErr, "thing error", []byte(`{"wrong": "value"}`))

	sr.Add(QueueFullErr, 23)

	jsonByte, err := sr.Marshal()
	require.NoError(t, err)

	var jsonOut map[string]interface{}
	err = json.Unmarshal(jsonByte, &jsonOut)
	require.NoError(t, err)

	verifyErr := tests.ApproveJson(jsonOut, "approved-stream-response/testStreamResponseAdvanced", nil)
	if verifyErr != nil {
		assert.Fail(t, fmt.Sprintf("Test %s failed with error: %s", "testStreamResponseAdvanced", verifyErr.Error()))
	}

	expectedStr := `queue is full (23), validation error (5): transmogrifier error ({"wrong": "field"}), thing error ({"wrong": "value"})`
	assert.Equal(t, expectedStr, sr.String())

	assert.Equal(t, 429, sr.StatusCode())
}
