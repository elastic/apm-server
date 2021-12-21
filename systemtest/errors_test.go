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

package systemtest_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestErrorIngest(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServer(t)

	// TODO(marclop): Update APM go agent to the latest to test this.
	events, err := ioutil.ReadFile("../testdata/intake-v2/errors.ndjson")
	require.NoError(t, err)

	req, _ := http.NewRequest("POST", srv.URL+"/intake/v2/events", bytes.NewReader(events))
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	result := systemtest.Elasticsearch.ExpectDocs(t, "logs-apm.error*", estest.ExistsQuery{
		Field: "transaction.name",
	})
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
}
