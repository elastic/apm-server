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

package asserts

import (
	"encoding/json"
	"testing"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// APMLogEntry is used to unmarshal RawJSON from search responses
// for displaying information.
type APMLogEntry struct {
	Message   string
	LogLogger string `json:"log.logger"`
}

func ZeroAPMLogs(t *testing.T, resp search.Response) {
	t.Helper()

	// Total is present only if `track_total_hits` wasn't `false` in
	// the search request. Guard against an unexpected panic.
	// This may also happen if the fixture does not include it.
	if resp.Hits.Total == nil {
		panic("hits.total.value is missing, are you setting track_total_hits=false in the request or is it missing from the fixture?")
	}

	// if there are no error logs, we are done here.
	if resp.Hits.Total.Value == 0 {
		return
	}

	// otherwise unmarshal for display.
	logs := []APMLogEntry{}
	for _, v := range resp.Hits.Hits {
		var l APMLogEntry
		require.NoError(t, json.Unmarshal(v.Source_, &l))
		logs = append(logs, l)
	}

	if !assert.Len(t, logs, 0, "expected no error logs, but found some") {
		t.Log("found error logs:")
		for _, l := range logs {
			t.Logf("- (%s) %s", l.LogLogger, l.Message)
		}
	}
}
