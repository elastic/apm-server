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

// loggerESlog is used to unmarshal RawJSON from search responses
// to allow filtering on specific fields.
type loggerESlog struct {
	Message       string
	Elasticsearch struct {
		Server struct {
			ErrorMessage string `json:"error.message"`
		}
	}
	Log struct {
		Logger string
	}
}

// includeLog returns a boolean indicating if the specified log
// should be included in further processing.
func includeLog(l loggerESlog) bool {
	switch l.Log.Logger {
	case "org.elasticsearch.ingest.geoip.GeoIpDownloader":
		return false
	}

	return true
}

func ZeroESLogs(t *testing.T, resp search.Response) {
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

	// otherwise apply filters an assert what's left
	logs := []loggerESlog{}
	for _, v := range resp.Hits.Hits {
		var l loggerESlog
		require.NoError(t, json.Unmarshal(v.Source_, &l))

		if includeLog(l) {
			logs = append(logs, l)
		}
	}

	if !assert.Len(t, logs, 0, "expected no error logs, but found some") {
		// As most of the errors we face here are not related to APM Server
		// and are not consistently appearing, we need a way to easily extract
		// the response for fitlering it out. Response is then used in tests.
		t.Log("printing response for adding test cases:")
		r, err := json.Marshal(resp)
		require.NoError(t, err, "cannot marshal response")
		t.Log(string(r))

		t.Log("found error logs:")
		for _, l := range logs {
			t.Logf("- (%s) %s: %s", l.Log.Logger, l.Message, l.Elasticsearch.Server.ErrorMessage)
		}
	}
}
