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
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/kibana"
	"github.com/elastic/apm-server/tests"
)

type m map[string]interface{}

var (
	testExp     = time.Nanosecond
	mockVersion = *common.MustNewVersion("7.3.0")
)

func TestFetcher_Fetch(t *testing.T) {
	t.Run("ErrorInput", func(t *testing.T) {
		kerr := errors.New("test error")
		_, _, ferr := NewFetcher(&kibana.ConnectingClient{}, testExp).Fetch(query(t.Name()), kerr)
		require.Error(t, ferr)
		assert.Equal(t, kerr, ferr)
	})

	t.Run("FetchError", func(t *testing.T) {
		kb := tests.MockKibana(http.StatusMultipleChoices, m{"error": "an error"}, mockVersion, true)
		_, _, err := NewFetcher(kb, testExp).Fetch(query(t.Name()), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), ErrMsgMultipleChoices)
	})

	t.Run("ExpectationFailed", func(t *testing.T) {
		kb := tests.MockKibana(http.StatusExpectationFailed, m{"error": "an error"}, mockVersion, true)
		_, _, err := NewFetcher(kb, testExp).Fetch(query(t.Name()), nil)
		require.Error(t, err)
		assert.Equal(t, "{\"error\":\"an error\"}", err.Error())
	})

	t.Run("NotFound", func(t *testing.T) {
		kb := tests.MockKibana(http.StatusNotFound, m{}, mockVersion, true)
		doc, err := NewDoc([]byte{})
		require.NoError(t, err)

		result, etag, err := NewFetcher(kb, testExp).Fetch(query(t.Name()), nil)
		require.NoError(t, err)
		assert.Equal(t, doc.ID, etag)
		assert.Equal(t, doc.Settings, Settings(result))
	})

	t.Run("Success", func(t *testing.T) {
		kb := tests.MockKibana(http.StatusOK, mockDoc(0.5), mockVersion, true)
		b, err := json.Marshal(mockDoc(0.5))
		require.NoError(t, err)
		doc, err := NewDoc(b)
		require.NoError(t, err)

		result, etag, err := NewFetcher(kb, testExp).Fetch(query(t.Name()), nil)
		require.NoError(t, err)
		assert.Equal(t, doc.ID, etag)
		assert.Equal(t, doc.Settings, Settings(result))
	})

	t.Run("FetchFromCache", func(t *testing.T) {

		fetch := func(f *Fetcher, kibanaSamplingRate, expectedSamplingRate float64) {

			client := func(samplingRate float64) kibana.Client {
				return tests.MockKibana(http.StatusOK, mockDoc(samplingRate), mockVersion, true)
			}
			f.kbClient = client(kibanaSamplingRate)

			b, err := json.Marshal(mockDoc(expectedSamplingRate))
			require.NoError(t, err)
			doc, err := NewDoc(b)
			require.NoError(t, err)

			result, etag, err := f.Fetch(query(t.Name()), nil)
			require.NoError(t, err)
			assert.Equal(t, doc.ID, etag)
			assert.Equal(t, doc.Settings, Settings(result))
		}

		fetcher := NewFetcher(nil, time.Minute)

		// nothing cached yet
		fetch(fetcher, 0.5, 0.5)

		// next fetch runs against cache
		fetch(fetcher, 0.8, 0.5)

		// after key is expired, fetch from Kibana again
		fetcher.docCache.gocache.Delete(query(t.Name()).ID())
		fetch(fetcher, 0.7, 0.7)

	})
}

func query(name string) Query {
	return Query{Service: Service{Name: name}}
}

func mockDoc(sampleRate float64) m {
	return m{
		"_id": "1",
		"_source": m{
			"settings": m{
				"sampling_rate": sampleRate,
			},
		},
	}
}
