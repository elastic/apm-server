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
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func SendRUMEventsPayload(t *testing.T, srv *apmservertest.Server, payloadFile string) {
	f := openFile(t, payloadFile)
	sendEventsPayload(t, srv, "/intake/v2/rum/events", f)
}

func SendRUMEventsLiteral(t *testing.T, srv *apmservertest.Server, raw string) {
	sendEventsPayload(t, srv, "/intake/v2/rum/events", strings.NewReader(raw))
}

func SendBackendEventsPayload(t *testing.T, srv *apmservertest.Server, payloadFile string) {
	f := openFile(t, payloadFile)
	sendEventsPayload(t, srv, "/intake/v2/events", f)
}

func SendBackendEventsAsyncPayload(t *testing.T, srv *apmservertest.Server, payloadFile string) {
	f := openFile(t, payloadFile)
	sendEventsPayload(t, srv, "/intake/v2/events?async=true", f)
}

func SendBackendEventsAsyncPayloadError(t *testing.T, srv *apmservertest.Server, payloadFile string) {
	f := openFile(t, payloadFile)

	resp := doRequest(t, srv, "/intake/v2/events?async=true", f)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, string(respBody))
}

func SendBackendEventsLiteral(t *testing.T, srv *apmservertest.Server, raw string) {
	sendEventsPayload(t, srv, "/intake/v2/events", strings.NewReader(raw))
}

func sendEventsPayload(t *testing.T, srv *apmservertest.Server, urlPath string, f io.Reader) {
	t.Helper()
	resp := doRequest(t, srv, urlPath, f)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp.StatusCode, string(respBody))
}

func doRequest(t *testing.T, srv *apmservertest.Server, urlPath string, f io.Reader) *http.Response {
	req, _ := http.NewRequest("POST", srv.URL+urlPath, f)
	req.Header.Add("Content-Type", "application/x-ndjson")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func openFile(t *testing.T, p string) *os.File {
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	} else {
		t.Cleanup(func() {
			f.Close()
		})
	}
	return f
}
