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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
)

func TestTransactionSetResetIsSet(t *testing.T) {
	var tRoot transactionRoot
	modeldecodertest.DecodeTestData(t, reader(t, "transactions"), "transaction", &tRoot)
	require.True(t, tRoot.IsSet())
	// call Reset and ensure initial state, except for array capacity
	tRoot.Reset()
	assert.False(t, tRoot.IsSet())
}

func TestTransactionValidationRules(t *testing.T) {
	testTransaction := func(t *testing.T, key string, tc testcase) {
		var event transaction
		r := reader(t, "transactions")
		modeldecodertest.ReplaceTestData(t, r, "transaction", key, tc.data, &event)

		// run validation and checks
		err := event.validate()
		if tc.errorKey == "" {
			assert.NoError(t, err)
		} else {
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorKey)
		}
	}

	t.Run("context", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "custom", data: `{"custom":{"k1":{"v1":123,"v2":"value"},"k2":34,"k3":[{"a.1":1,"b*\"":2}]}}`},
			{name: "custom-key-dot", errorKey: "patternKeys", data: `{"custom":{"k1.":{"v1":123,"v2":"value"}}}`},
			{name: "custom-key-asterisk", errorKey: "patternKeys", data: `{"custom":{"k1*":{"v1":123,"v2":"value"}}}`},
			{name: "custom-key-quote", errorKey: "patternKeys", data: `{"custom":{"k1\"":{"v1":123,"v2":"value"}}}`},
			{name: "tags", data: `{"tags":{"k1":"v1.s*\"","k2":34,"k3":23.56,"k4":true}}`},
			{name: "tags-key-dot", errorKey: "patternKeys", data: `{"tags":{"k1.":"v1"}}`},
			{name: "tags-key-asterisk", errorKey: "patternKeys", data: `{"tags":{"k1*":"v1"}}`},
			{name: "tags-key-quote", errorKey: "patternKeys", data: `{"tags":{"k1\"":"v1"}}`},
			{name: "tags-invalid-type", errorKey: "typesVals", data: `{"tags":{"k1":{"v1":"abc"}}}`},
			{name: "tags-invalid-type", errorKey: "typesVals", data: `{"tags":{"k1":{"v1":[1,2,3]}}}`},
			{name: "tags-maxVal", data: `{"tags":{"k1":"` + modeldecodertest.BuildString(1024) + `"}}`},
			{name: "tags-maxVal-exceeded", errorKey: "maxVals", data: `{"tags":{"k1":"` + modeldecodertest.BuildString(1025) + `"}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "context", tc)
			})
		}
	})

	// this tests an arbitrary field to ensure the max rule works as expected
	t.Run("max", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "context-message-queue-name", data: `{"message":{"queue":{"name":"` + modeldecodertest.BuildString(1024) + `"}}}`},
			{name: "context-message-queue-name", errorKey: "max", data: `{"message":{"queue":{"name":"` + modeldecodertest.BuildString(1025) + `"}}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "context", tc)
			})
		}
	})

	t.Run("request", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "request-body-string", data: `"body":"value"`},
			{name: "request-body-object", data: `"body":{"a":"b"}`},
			{name: "request-body-array", errorKey: "transaction.context.request.body", data: `"body":[1,2]`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tc.data = `{"request":{"method":"get",` + tc.data + `}}`
				testTransaction(t, "context", tc)
			})
		}
	})

	t.Run("service", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "service-name-az", data: `{"service":{"name":"abcdefghijklmnopqrstuvwxyz"}}`},
			{name: "service-name-AZ", data: `{"service":{"name":"ABCDEFGHIJKLMNOPQRSTUVWXYZ"}}`},
			{name: "service-name-09 _-", data: `{"service":{"name":"0123456789 -_"}}`},
			{name: "service-name-invalid", errorKey: "regexpAlphaNumericExt", data: `{"service":{"name":"⌘"}}`},
			{name: "service-name-max", data: `{"service":{"name":"` + modeldecodertest.BuildStringWith(1024, '-') + `"}}`},
			{name: "service-name-max-exceeded", errorKey: "max", data: `{"service":{"name":"` + modeldecodertest.BuildStringWith(1025, '-') + `"}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "context", tc)
			})
		}
	})

	t.Run("duration", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "duration", data: `0.0`},
			{name: "duration", errorKey: "min", data: `-0.09`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "duration", tc)
			})
		}
	})

	t.Run("marks", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "marks", data: `{"k1":{"v1":12.3}}`},
			{name: "marks-dot", errorKey: "patternKeys", data: `{"k.1":{"v1":12.3}}`},
			{name: "marks-dot", errorKey: "patternKeys", data: `{"k1":{"v.1":12.3}}`},
			{name: "marks-asterisk", errorKey: "patternKeys", data: `{"k*1":{"v1":12.3}}`},
			{name: "marks-asterisk", errorKey: "patternKeys", data: `{"k1":{"v*1":12.3}}`},
			{name: "marks-quote", errorKey: "patternKeys", data: `{"k\"1":{"v1":12.3}}`},
			{name: "marks-quote", errorKey: "patternKeys", data: `{"k1":{"v\"1":12.3}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "marks", tc)
			})
		}
	})

	t.Run("outcome", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "outcome-success", data: `"success"`},
			{name: "outcome-failure", data: `"failure"`},
			{name: "outcome-unknown", data: `"unknown"`},
			{name: "outcome-invalid", errorKey: "enum", data: `"anything"`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "outcome", tc)
			})
		}
	})

	t.Run("url", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "port-string", data: `"port":"8200"`},
			{name: "port-int", data: `"port":8200`},
			{name: "port-invalid-type", errorKey: "types", data: `"port":[8200,8201]`},
			{name: "port-invalid-type", errorKey: "types", data: `"port":{"val":8200}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tc.data = `{"request":{"method":"get","url":{` + tc.data + `}}}`
				testTransaction(t, "context", tc)
			})
		}
	})

	t.Run("user", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "id-string", data: `{"user":{"id":"user123"}}`},
			{name: "id-int", data: `{"user":{"id":44}}`},
			{name: "id-float", errorKey: "types", data: `{"user":{"id":45.6}}`},
			{name: "id-bool", errorKey: "types", data: `{"user":{"id":true}}`},
			{name: "id-string-max-len", data: `{"user":{"id":"` + modeldecodertest.BuildString(1024) + `"}}`},
			{name: "id-string-max-len-exceeded", errorKey: "max", data: `{"user":{"id":"` + modeldecodertest.BuildString(1025) + `"}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "context", tc)
			})
		}
	})

	t.Run("required", func(t *testing.T) {
		// setup: create full metadata struct with arbitrary values set
		var event transaction
		modeldecodertest.InitStructValues(&event)
		// test vanilla struct is valid
		require.NoError(t, event.validate())

		// iterate through struct, remove every key one by one
		// and test that validation behaves as expected
		requiredKeys := map[string]interface{}{
			"duration":               nil,
			"id":                     nil,
			"span_count":             nil,
			"span_count.started":     nil,
			"trace_id":               nil,
			"type":                   nil,
			"context.request.method": nil,
		}
		modeldecodertest.SetZeroStructValue(&event, func(key string) {
			err := event.validate()
			if _, ok := requiredKeys[key]; ok {
				require.Error(t, err, key)
				assert.Contains(t, err.Error(), key)
			} else {
				assert.NoError(t, err, key)
			}
		})
	})
}
