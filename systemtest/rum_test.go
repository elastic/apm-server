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
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestRUMXForwardedFor(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	serverURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	serverURL.Path = "/intake/v2/rum/events"

	const body = `{"metadata":{"service":{"name":"rum-js-test","agent":{"name":"rum-js","version":"5.5.0"}}}}
{"transaction":{"trace_id":"611f4fa950f04631aaaaaaaaaaaaaaaa","id":"611f4fa950f04631","type":"page-load","duration":643,"span_count":{"started":0}}}`

	req, _ := http.NewRequest("POST", serverURL.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("X-Forwarded-For", "220.244.41.16")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*", estest.TermQuery{Field: "processor.event", Value: "transaction"})
	systemtest.ApproveEvents(
		t, t.Name(), result.Hits.Hits,
		// RUM timestamps are set by the server based on the time the payload is received.
		"@timestamp", "timestamp.us",
	)
}

func TestRUMErrorSourcemapping(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	uploadSourcemap(t, srv, "../testdata/sourcemap/bundle.js.map",
		"http://localhost:8000/test/e2e/../e2e/general-usecase/bundle.js.map", // bundle filepath
		"apm-agent-js", // service name
		"1.0.1",        // service version
	)
	systemtest.Elasticsearch.ExpectDocs(t, "apm-*-sourcemap", nil)

	sendRUMEventsPayload(t, srv, "../testdata/intake-v2/errors_rum.ndjson")
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*-error", nil)

	systemtest.ApproveEvents(
		t, t.Name(), result.Hits.Hits,
		// RUM timestamps are set by the server based on the time the payload is received.
		"@timestamp", "timestamp.us",
	)
}

func TestRUMAuth(t *testing.T) {
	// The RUM endpoint does not require auth. Start the server
	// with a randomly generated secret token to show that RUM
	// events can be sent without passing the secret token.
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	secretToken := strconv.Itoa(rng.Int())

	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.SecretToken = secretToken
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	sendRUMEventsPayload(t, srv, "../testdata/intake-v2/transactions.ndjson")

	req, _ := http.NewRequest("GET", srv.URL+"/config/v1/rum/agents", nil)
	req.Header.Add("Content-Type", "application/json")
	req.URL.RawQuery = url.Values{"service.name": []string{"service_name"}}.Encode()
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRUMAllowServiceNames(t *testing.T) {
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.SecretToken = "abc123"
	srv.Config.RUM = &apmservertest.RUMConfig{
		Enabled:           true,
		AllowServiceNames: []string{"allowed"},
	}
	err := srv.Start()
	require.NoError(t, err)

	// Send a RUM transaction where the service name in metadata is allowed,
	// but is overridden in the transaction event's context with a disallowed
	// service name.
	reqBody := strings.NewReader(`
{"metadata":{"service":{"name":"allowed","version":"1.0.0","agent":{"name":"rum-js","version":"0.0.0"}}}}
{"transaction":{"trace_id":"x","id":"y","type":"z","duration":0,"span_count":{"started":1},"context":{"service":{"name":"disallowed"}}}}
`[1:])
	req, _ := http.NewRequest("POST", srv.URL+"/intake/v2/rum/events", reqBody)
	req.Header.Add("Content-Type", "application/x-ndjson")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, string(respBody))
	assert.Equal(t, `{"accepted":0,"errors":[{"message":"service name is not allowed"}]}`+"\n", string(respBody))
}

func TestRUMRateLimit(t *testing.T) {
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{
		Enabled: true,
		RateLimit: &apmservertest.RUMRateLimitConfig{
			IPLimit:    2,
			EventLimit: 10,
		},
	}
	err := srv.Start()
	require.NoError(t, err)

	sendEvents := func(ip string, n int) error {
		body := bytes.NewBufferString(`{"metadata":{"service":{"name":"allowed","version":"1.0.0","agent":{"name":"rum-js","version":"0.0.0"}}}}` + "\n")
		for i := 0; i < n; i++ {
			body.WriteString(`{"transaction":{"trace_id":"x","id":"y","type":"z","duration":0,"span_count":{"started":1},"context":{"service":{"name":"foo"}}}}` + "\n")
		}

		req, _ := http.NewRequest("POST", srv.URL+"/intake/v2/rum/events?verbose=true", body)
		req.Header.Add("Content-Type", "application/x-ndjson")
		req.Header.Add("X-Forwarded-For", ip)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusAccepted {
			return fmt.Errorf("%s (%s)", resp.Status, strings.TrimSpace(string(respBody)))
		}
		return nil
	}

	// Just check that rate limiting is wired up. More specific rate limiting scenarios are unit tested.

	var g errgroup.Group
	g.Go(func() error { return sendEvents("10.11.12.13", srv.Config.RUM.RateLimit.EventLimit) })
	g.Go(func() error { return sendEvents("10.11.12.14", srv.Config.RUM.RateLimit.EventLimit) })
	assert.NoError(t, g.Wait())

	g = errgroup.Group{}
	g.Go(func() error { return sendEvents("10.11.12.13", srv.Config.RUM.RateLimit.EventLimit) })
	g.Go(func() error { return sendEvents("10.11.12.14", srv.Config.RUM.RateLimit.EventLimit) })
	g.Go(func() error { return sendEvents("10.11.12.15", srv.Config.RUM.RateLimit.EventLimit) })
	assert.EqualError(t, g.Wait(), `429 Too Many Requests ({"accepted":0,"errors":[{"message":"rate limit exceeded"}]})`)
}

func sendRUMEventsPayload(t *testing.T, srv *apmservertest.Server, payloadFile string) {
	t.Helper()

	f, err := os.Open(payloadFile)
	require.NoError(t, err)
	defer f.Close()

	req, _ := http.NewRequest("POST", srv.URL+"/intake/v2/rum/events", f)
	req.Header.Add("Content-Type", "application/x-ndjson")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp.StatusCode, string(respBody))
}

func uploadSourcemap(t *testing.T, srv *apmservertest.Server, sourcemapFile, bundleFilepath, serviceName, serviceVersion string) {
	t.Helper()

	req := newUploadSourcemapRequest(t, srv, sourcemapFile, bundleFilepath, serviceName, serviceVersion)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp.StatusCode, string(respBody))
}

func newUploadSourcemapRequest(t *testing.T, srv *apmservertest.Server, sourcemapFile, bundleFilepath, serviceName, serviceVersion string) *http.Request {
	t.Helper()

	var data bytes.Buffer
	mw := multipart.NewWriter(&data)
	require.NoError(t, mw.WriteField("service_name", serviceName))
	require.NoError(t, mw.WriteField("service_version", serviceVersion))
	require.NoError(t, mw.WriteField("bundle_filepath", bundleFilepath))

	f, err := os.Open(sourcemapFile)
	require.NoError(t, err)
	defer f.Close()
	sourcemapFileWriter, err := mw.CreateFormFile("sourcemap", filepath.Base(sourcemapFile))
	require.NoError(t, err)
	_, err = io.Copy(sourcemapFileWriter, f)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req, _ := http.NewRequest("POST", srv.URL+"/assets/v1/sourcemaps", &data)
	req.Header.Add("Content-Type", mw.FormDataContentType())
	return req
}
