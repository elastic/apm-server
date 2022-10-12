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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-agent-libs/mapstr"
)

func TestLogTransform(t *testing.T) {
	tests := []struct {
		Log    Log
		Output mapstr.M
	}{
		{
			Log:    Log{},
			Output: nil,
		},
		{
			Log: Log{
				Level:  "warn",
				Logger: "bootstrap",
			},
			Output: mapstr.M{
				"level":  "warn",
				"logger": "bootstrap",
			},
		},
		{
			Log: Log{
				Level:  "warn",
				Logger: "bootstrap",
				Origin: &LogOrigin{
					LogFile: &LogOriginFile{
						Name: "testFile",
						Line: 12,
					},
					FunctionName: "testFunc",
				},
			},
			Output: mapstr.M{
				"level":  "warn",
				"logger": "bootstrap",
				"origin": mapstr.M{
					"function": "testFunc",
					"file": mapstr.M{
						"name": "testFile",
						"line": 12,
					},
				},
			},
		},
	}

	for _, test := range tests {
		output := test.Log.fields()
		assert.Equal(t, test.Output, output)
	}
}
