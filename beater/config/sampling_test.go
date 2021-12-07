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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestSamplingPoliciesValidation(t *testing.T) {
	t.Run("MinimallyValid", func(t *testing.T) {
		_, err := NewConfig(common.MustNewConfigFrom(map[string]interface{}{
			"sampling.tail.policies": []map[string]interface{}{{
				"sample_rate": 0.5,
			}},
		}), nil)
		assert.NoError(t, err)
	})
	t.Run("NoPolicies", func(t *testing.T) {
		_, err := NewConfig(common.MustNewConfigFrom(map[string]interface{}{
			"sampling.tail.enabled": true,
		}), nil)
		assert.EqualError(t, err, "Error processing configuration: invalid tail sampling config: no policies specified accessing 'sampling.tail'")
	})
	t.Run("NoDefaultPolicies", func(t *testing.T) {
		_, err := NewConfig(common.MustNewConfigFrom(map[string]interface{}{
			"sampling.tail.policies": []map[string]interface{}{{
				"service.name": "foo",
				"sample_rate":  0.5,
			}},
		}), nil)
		assert.EqualError(t, err, "Error processing configuration: invalid tail sampling config: no default (empty criteria) policy specified accessing 'sampling.tail'")
	})
}
