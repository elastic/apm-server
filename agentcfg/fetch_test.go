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
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/kibana"
	"github.com/elastic/apm-server/kibana/kibanatest"
)

type m map[string]interface{}

var (
	testExpiration = time.Nanosecond
	mockVersion    = *common.MustNewVersion("7.3.0")
)

func TestFetcher_Fetch(t *testing.T) {

	t.Run("ExpectationFailed", func(t *testing.T) {
<<<<<<< HEAD
		kb := tests.MockKibana(http.StatusExpectationFailed, m{"error": "an error"}, mockVersion, true)
		_, err := NewFetcher(kb, testExpiration).Fetch(context.Background(), query(t.Name()))
=======
		kb := kibanatest.MockKibana(http.StatusExpectationFailed, m{"error": "an error"}, mockVersion, true)
		_, err := NewKibanaFetcher(kb, testExpiration).Fetch(context.Background(), query(t.Name()))
>>>>>>> cb20a978 (Move tests.MockKibanaClient to its own package (#5273))
		require.Error(t, err)
		assert.Equal(t, "{\"error\":\"an error\"}", err.Error())
	})

	t.Run("NotFound", func(t *testing.T) {
<<<<<<< HEAD
		kb := tests.MockKibana(http.StatusNotFound, m{}, mockVersion, true)
		result, err := NewFetcher(kb, testExpiration).Fetch(context.Background(), query(t.Name()))
=======
		kb := kibanatest.MockKibana(http.StatusNotFound, m{}, mockVersion, true)
		result, err := NewKibanaFetcher(kb, testExpiration).Fetch(context.Background(), query(t.Name()))
>>>>>>> cb20a978 (Move tests.MockKibanaClient to its own package (#5273))
		require.NoError(t, err)
		assert.Equal(t, zeroResult(), result)
	})

	t.Run("Success", func(t *testing.T) {
		kb := kibanatest.MockKibana(http.StatusOK, mockDoc(0.5), mockVersion, true)
		b, err := json.Marshal(mockDoc(0.5))
		expectedResult, err := newResult(b, err)
		require.NoError(t, err)
		result, err := NewFetcher(kb, testExpiration).Fetch(context.Background(), query(t.Name()))
		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("FetchFromCache", func(t *testing.T) {

		fetch := func(f *Fetcher, kibanaSamplingRate, expectedSamplingRate float64) {

			client := func(samplingRate float64) kibana.Client {
				return kibanatest.MockKibana(http.StatusOK, mockDoc(samplingRate), mockVersion, true)
			}
			f.client = client(kibanaSamplingRate)

			b, err := json.Marshal(mockDoc(expectedSamplingRate))
			require.NoError(t, err)
			expectedResult, err := newResult(b, err)
			require.NoError(t, err)

			result, err := f.Fetch(context.Background(), query(t.Name()))
			require.NoError(t, err)
			assert.Equal(t, expectedResult, result)
		}

		fetcher := NewFetcher(nil, time.Minute)

		// nothing cached yet
		fetch(fetcher, 0.5, 0.5)

		// next fetch runs against cache
		fetch(fetcher, 0.8, 0.5)

		// after key is expired, fetch from Kibana again
		fetcher.cache.gocache.Delete(query(t.Name()).id())
		fetch(fetcher, 0.7, 0.7)

	})
}

func TestSanitize(t *testing.T) {
	input := Result{Source: Source{
		Agent:    "python",
		Settings: Settings{"transaction_sample_rate": "0.1", "capture_body": "false"}}}
	// full result as not requested for an insecure agent
	assert.Equal(t, input, sanitize([]string{}, input))

	// no result for insecure agent
	assert.Equal(t, zeroResult(), sanitize([]string{"rum-js"}, input))

	// limited result for insecure agent
	insecureAgents := []string{"rum-js"}
	input.Source.Agent = "rum-js"
	assert.Equal(t, Settings{"transaction_sample_rate": "0.1"}, sanitize(insecureAgents, input).Source.Settings)

	// limited result for insecure agent prefix
	insecureAgents = []string{"Jaeger"}
	input.Source.Agent = "Jaeger/Python"
	assert.Equal(t, Settings{"transaction_sample_rate": "0.1"}, sanitize(insecureAgents, input).Source.Settings)

	// no result for insecure agent prefix
	insecureAgents = []string{"Python"}
	input.Source.Agent = "Jaeger/Python"
	assert.Equal(t, zeroResult(), sanitize(insecureAgents, input))
}

func TestCustomJSON(t *testing.T) {
	expected := Result{Source: Source{
		Etag:     "123",
		Settings: map[string]string{"transaction_sampling_rate": "0.3"}}}
	input := `{"_id": "1", "_source":{"etag":"123", "settings":{"transaction_sampling_rate": 0.3}}}`
	actual, _ := newResult([]byte(input), nil)
	assert.Equal(t, expected, actual)
}

func query(name string) Query {
	return Query{Service: Service{Name: name}, Etag: "123"}
}

func mockDoc(sampleRate float64) m {
	return m{
		"_id": "1",
		"_source": m{
			"settings": m{
				"sampling_rate": sampleRate,
			},
			"etag":       "123",
			"agent_name": "rum-js",
		},
	}
}
