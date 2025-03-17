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
	"net/url"
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
func getHttpClient(t *testing.T) (*recorder.Recorder, *http.Client) {
	t.Helper()

	var err error
	var rec *recorder.Recorder
	var hc *http.Client

	tr := http.DefaultTransport
	rec, err = recorder.NewAsMode(path.Join("testdata", "fixtures", t.Name()), recorder.ModeReplaying, tr)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := rec.Stop(); err != nil {
			t.Fatalf("cannot stop HTTP recorder: %s", err)
		}
	})

	// As we are replacing the url in saved data, we also need to use a
	// custom matcher to ignore the hostname.
	rec.SetMatcher(func(r *http.Request, i cassette.Request) bool {
		// copy to avoid changing request being handled
		u := *r.URL
		u.Host = "test"
		return r.Method == i.Method && u.String() == i.URL
	})

	// Filter out dynamic & sensitive data/headers before saving the fixture.
	// Tests will be using the real values when recording the interactions.
	rec.AddSaveFilter(func(i *cassette.Interaction) error {
		delete(i.Request.Headers, "Authorization")

		delete(i.Response.Headers, "Reporting-Endpoints")
		delete(i.Response.Headers, "Kbn-License-Sig")
		delete(i.Response.Headers, "X-Found-Handling-Cluster")

		var sanitizeBody = func(content string) string {
			newcontent := regexp.MustCompile(`"api_key":"[a-zA-Z0-9_\-\:]+"`).
				ReplaceAll([]byte(content), []byte(`"api_key":"REDACTED"`))
			newcontent = regexp.MustCompile(`"secret_token":{"type":"text","value":"[a-zA-Z0-9\-_\:]*"}`).
				ReplaceAll(newcontent, []byte(`"secret_token":{"type":"text","value":"REDACTED"}`))
			newcontent = regexp.MustCompile(`"secret_token":"[a-zA-Z0-9\-_\:]*"`).
				ReplaceAll(newcontent, []byte(`"secret_token":"REDACTED"`))
			newcontent = regexp.MustCompile(`"url":{(.*?)"value":"https:\/\/[a-z\-0-9\.]*\:[0-9]*"(.*?)}`).
				ReplaceAll(newcontent, []byte(`"url":{$1"value":"https://test"$2}`))
			return string(newcontent)
		}

		i.Request.Body = sanitizeBody(i.Request.Body)
		i.Response.Body = sanitizeBody(i.Response.Body)

		var redactURL = func(u string) string {
			ur, err := url.Parse(u)
			require.NoError(t, err)
			ur.Host = "test"
			return ur.String()
		}

		i.Request.URL = redactURL(i.Request.URL)

		return nil
	})

	hc = &http.Client{
		Transport: rec,
		Timeout:   10 * time.Second,
	}

	return rec, hc
}

func TestGetPackagePolicy(t *testing.T) {
	kibanaURL := os.Getenv("KIBANA_URL")
	apikey := os.Getenv("KIBANA_APIKEY")
	c, err := kbclient.New(kibanaURL, apikey)
	require.NoError(t, err)
	_, httpc := getHttpClient(t)
	c.Client = *httpc

	p := "elastic-cloud-apm"
	_, err = c.GetPackagePolicyByID(p)
	require.NoError(t, err)
}

func TestUpdatePackagePolicy(t *testing.T) {
	kibanaURL := os.Getenv("KIBANA_URL")
	apikey := os.Getenv("KIBANA_APIKEY")
	c, err := kbclient.New(kibanaURL, apikey)
	require.NoError(t, err)
	_, httpc := getHttpClient(t)
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
