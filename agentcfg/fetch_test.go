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
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/kibana"

	"github.com/elastic/apm-server/tests"
)

type m map[string]interface{}

var testExp = time.Nanosecond

func query(name string) Query {
	return Query{Service: Service{Name: name}}
}

func TestFetchNoClient(t *testing.T) {
	kb, kerr := kibana.NewKibanaClient(nil)
	_, _, ferr := NewFetcher(kb, testExp).Fetch(query(t.Name()), kerr)
	require.Error(t, ferr)
	assert.Equal(t, kerr, ferr)
}

func TestFetchStringConversion(t *testing.T) {
	kb := tests.MockKibana(http.StatusOK,
		m{
			"_id": "1",
			"_source": m{
				"settings": m{
					"sampling_rate": 0.5,
				},
			},
		})
	result, etag, err := NewFetcher(kb, testExp).Fetch(query(t.Name()), nil)
	require.NoError(t, err)
	assert.Equal(t, "1", etag, etag)
	assert.Equal(t, map[string]string{"sampling_rate": "0.5"}, result)
}

func TestFetchVersionCheck(t *testing.T) {
	kb := tests.MockKibana(http.StatusOK, m{})
	kb.Connection.Version.Major = 6
	_, _, err := NewFetcher(kb, testExp).Fetch(query(t.Name()), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestFetchError(t *testing.T) {
	kb := tests.MockKibana(http.StatusExpectationFailed, m{"error": "an error"})
	_, _, err := NewFetcher(kb, testExp).Fetch(query(t.Name()), nil)
	require.Error(t, err)
	assert.Equal(t, "{\"error\":\"an error\"}", err.Error())
}

func TestFetchWithCaching(t *testing.T) {
	fetch := func(f *Fetcher, samplingRate float64) map[string]string {

		client := func(samplingRate float64) *kibana.Client {
			return tests.MockKibana(http.StatusOK,
				m{
					"_id": "1",
					"_source": m{
						"settings": m{
							"sampling_rate": samplingRate,
						},
					},
				})
		}
		f.kbClient = client(samplingRate)

		result, id, err := f.Fetch(query(t.Name()), nil)
		require.NoError(t, err)
		require.Equal(t, "1", id)
		return result
	}

	fetcher := NewFetcher(nil, time.Minute)

	// nothing cached yet
	result := fetch(fetcher, 0.5)
	assert.Equal(t, map[string]string{"sampling_rate": "0.5"}, result)

	// next fetch runs against cache
	result = fetch(fetcher, 0.8)
	assert.Equal(t, map[string]string{"sampling_rate": "0.5"}, result)

	// after key is expired, fetch from Kibana again
	fetcher.docCache.gocache.Delete(query(t.Name()).id())
	result = fetch(fetcher, 0.7)
	assert.Equal(t, map[string]string{"sampling_rate": "0.7"}, result)

}
