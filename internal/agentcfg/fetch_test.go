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

package agentcfg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	testExpiration = time.Nanosecond
)

func TestCustomJSON(t *testing.T) {
	expected := Result{Source: Source{
		Etag:     "123",
		Settings: map[string]string{"transaction_sampling_rate": "0.3"}}}
	input := `{"_id": "1", "_source":{"etag":"123", "settings":{"transaction_sampling_rate": 0.3}}}`
	actual, _ := newResult([]byte(input), nil)
	assert.Equal(t, expected, actual)
}
