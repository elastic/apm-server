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

package kbclient_test

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"testing"
	"time"

	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/itchyny/gojq"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
)

// getHttpClient instantiate a http.Client backed by a recorder.Recorder to be used in testing
// scenarios.
// To update any fixture, delete it from the filesystem and run the test again.
// NOTE: always remember to call (recorder.Recorder).Stop() in your test case.
func getHttpClient(t *testing.T) (*recorder.Recorder, *http.Client) {
	t.Helper()

	var err error
	var rec *recorder.Recorder
	var hc *http.Client

	tr := http.DefaultTransport
	rec, err = recorder.NewAsMode(path.Join("testdata", "fixtures", t.Name()), recorder.ModeReplaying, tr)
	require.NoError(t, err)

	// Filter out dynamic & sensitive data/headers before saving the fixture.
	// Tests will be using the real values when recording the interactions.
	rec.AddSaveFilter(func(i *cassette.Interaction) error {
		delete(i.Request.Headers, "Authorization")

		var sanitizeBody = func(content string) string {
			newcontent := regexp.MustCompile(`"api_key":"[a-zA-Z0-9_\-\:]+"`).
				ReplaceAll([]byte(content), []byte(`"api_key":"REDACTED"`))
			newcontent = regexp.MustCompile(`"secret_token":{"type":"text","value":"[a-zA-Z0-9\-_\:]+"}`).
				ReplaceAll(newcontent, []byte(`"secret_token":{"type":"text","value":"REDACTED"}`))
			newcontent = regexp.MustCompile(`"secret_token":"[a-zA-Z0-9\-_\:]+"`).
				ReplaceAll(newcontent, []byte(`"secret_token":"REDACTED"`))
			return string(newcontent)
		}

		i.Request.Body = sanitizeBody(i.Request.Body)
		i.Response.Body = sanitizeBody(i.Response.Body)

		return nil
	})

	hc = &http.Client{
		Transport: rec,
		Timeout:   10 * time.Second,
	}

	return rec, hc
}

func TestClientNew(t *testing.T) {
	kibanaURL := "http://example.com"
	apikey := "apikey"
	c := kbclient.New(kibanaURL, apikey)
	rec, httpc := getHttpClient(t)
	defer rec.Stop()
	c.Client = *httpc

	req, err := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	require.NoError(t, err)
	_, err = c.Do(req)
	require.NoError(t, err)
}

func TestGetPackagePolicy(t *testing.T) {
	kibanaURL := os.Getenv("KIBANA_URL")
	apikey := os.Getenv("KIBANA_APIKEY")
	c := kbclient.New(kibanaURL, apikey)
	rec, httpc := getHttpClient(t)
	defer rec.Stop()
	c.Client = *httpc

	p := "elastic-cloud-apm"
	_, err := c.GetPackagePolicyByID(p)
	require.NoError(t, err)
}

func TestUpdatePackagePolicy(t *testing.T) {
	kibanaURL := os.Getenv("KIBANA_URL")
	apikey := os.Getenv("KIBANA_APIKEY")
	c := kbclient.New(kibanaURL, apikey)
	rec, httpc := getHttpClient(t)
	defer rec.Stop()
	rec.Stop()
	c.Client = *httpc

	p := "elastic-cloud-apm"

	eca, err := c.GetPackagePolicyByID(p)
	require.NoError(t, err)

	var f interface{}
	require.NoError(t, json.Unmarshal(eca, &f))

	// these are the minimum modifications required for sending the
	// policy back to Fleet. Any less and you'll get a validation error.
	v := jq(t, ".item", f)
	v = jq(t, "del(.id) | del(.elasticsearch) | del(.inputs[].compiled_input) | del(.revision) | del(.created_at) | del(.created_by) | del(.updated_at) | del(.updated_by)", v)

	require.NoError(t, c.UpdatePackagePolicyByID("elastic-cloud-apm", v))

}

// jq runs a jq instruction on data. It expects a single object to be
// returned from the expression, as it does not iterate on the result
// set.
func jq(t *testing.T, q string, data any) any {
	query, err := gojq.Parse(q)
	if err != nil {
		log.Fatalln(err)
	}
	iter := query.Run(data)
	v, ok := iter.Next()
	require.True(t, ok)
	if err, ok := v.(error); ok {
		log.Fatalln(err)
	}
	return v
}
