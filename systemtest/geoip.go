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

package systemtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var stats struct {
	GeoIPStats `json:"stats"`
}

type GeoIPStats struct {
	DatabasesCount int `json:"databases_count"`
}

const requestBody = `{"metadata":{"service":{"name":"rum-js-test","agent":{"name":"rum-js","version":"5.5.0"}}}}
{"transaction":{"trace_id":"611f4fa950f04631aaaaaaaaaaaaaaaa","id":"611f4fa950f04631","type":"page-load","duration":643,"span_count":{"started":0}}}
{"metricset":{"samples":{"transaction.breakdown.count":{"value":12},"transaction.duration.sum.us":{"value":12},"transaction.duration.count":{"value":2},"transaction.self_time.sum.us":{"value":10},"transaction.self_time.count":{"value":2},"span.self_time.count":{"value":1},"span.self_time.sum.us":{"value":633.288}},"transaction":{"type":"request","name":"GET /"},"span":{"type":"external","subtype":"http"},"timestamp": 1496170422281000}}
`

func GeoIpLazyDownload(t *testing.T, url string) {
	t.Helper()

	req, err := http.NewRequest("POST", url, strings.NewReader(requestBody))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("X-Forwarded-For", "8.8.8.8")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	err = waitGeoIPDownload(t)
	require.NoError(t, err)

	// Clean all datastreams containing tags.
	CleanupElasticsearch(t)
}

func waitGeoIPDownload(t *testing.T) error {
	t.Helper()

	timer := time.NewTimer(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	first := true
	for {
		select {
		case <-ticker.C:
			resp, err := Elasticsearch.Ingest.GeoIPStats()
			require.NoError(t, err)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			err = json.Unmarshal([]byte(body), &stats)
			require.NoError(t, err)

			// [GeoLite2-ASN.mmdb, GeoLite2-City.mmdb, GeoLite2-Country.mmdb]
			if stats.DatabasesCount == 3 {
				t.Log("GeoIp database downloaded")
				return nil
			}
		case <-timer.C:
			return fmt.Errorf("download timeout exceeded")
		}

		if first {
			t.Log("waiting for GeoIP database to download")
			first = false
		}
	}
}
