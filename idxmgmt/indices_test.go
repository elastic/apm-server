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

package idxmgmt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventIdxNames(t *testing.T) {

	t.Run("ILM=false", func(t *testing.T) {
		expected := map[string]string{
			"error":       "apm-%{[observer.version]}-error-%{+yyyy.MM.dd}",
			"metric":      "apm-%{[observer.version]}-metric-%{+yyyy.MM.dd}",
			"profile":     "apm-%{[observer.version]}-profile-%{+yyyy.MM.dd}",
			"span":        "apm-%{[observer.version]}-span-%{+yyyy.MM.dd}",
			"transaction": "apm-%{[observer.version]}-transaction-%{+yyyy.MM.dd}",
		}

		assert.Equal(t, expected, eventIdxNames(true))
	})

	t.Run("ILM=true", func(t *testing.T) {
		expected := map[string]string{
			"error":       "apm-%{[observer.version]}-error",
			"metric":      "apm-%{[observer.version]}-metric",
			"profile":     "apm-%{[observer.version]}-profile",
			"span":        "apm-%{[observer.version]}-span",
			"transaction": "apm-%{[observer.version]}-transaction",
		}

		assert.Equal(t, expected, eventIdxNames(false))
	})
}
