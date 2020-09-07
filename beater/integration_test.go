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

package beater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	jsonoutput "github.com/elastic/beats/v7/libbeat/outputs/codec/json"
	"github.com/elastic/beats/v7/libbeat/version"

	"github.com/elastic/apm-server/beater/api"
	"github.com/elastic/apm-server/tests/approvals"
	"github.com/elastic/apm-server/tests/loader"
)

func collectEvents(events <-chan beat.Event, timeout time.Duration) []beat.Event {
	var collected []beat.Event
	for {
		select {
		case event := <-events:
			collected = append(collected, event)
		case <-time.After(timeout):
			return collected
		}
	}
}

var timestampOverride = time.Date(2019, 1, 9, 21, 40, 53, 690, time.UTC)

// adjustMissingTimestamp sets @timestamp and timestamp.us to known values for events that originally omitted a ts
func adjustMissingTimestamp(event *beat.Event) {
	if time.Since(event.Timestamp) < 5*time.Minute {
		event.Timestamp = timestampOverride
		if event.Fields["processor"].(common.MapStr)["name"] == "metric" {
			return
		}
		event.Fields.Put("timestamp.us", timestampOverride.UnixNano()/1000)
	}
}

// testPublishIntake exercises the publishing pipeline, from apm-server intake to beat publishing.
// It posts a payload to a running APM server via the intake API and gathers the resulting documents that would
// normally be published to Elasticsearch.
func testPublishIntake(t *testing.T, apm *testBeater, events <-chan beat.Event, payload io.Reader) []byte {
	req, err := http.NewRequest(http.MethodPost, apm.baseURL+api.IntakePath, payload)
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-ndjson")
	return testPublish(t, apm.client, req, events)
}

func testPublishProfile(t *testing.T, apm *testBeater, events <-chan beat.Event, metadata, profile io.Reader) []byte {
	var buf bytes.Buffer
	mpw := multipart.NewWriter(&buf)
	writePart := func(name, contentType string, body io.Reader) {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"`, name))
		h.Set("Content-Type", contentType)
		part, err := mpw.CreatePart(h)
		require.NoError(t, err)
		_, err = io.Copy(part, body)
		require.NoError(t, err)
	}
	if metadata != nil {
		writePart("metadata", "application/json", metadata)
	}
	writePart("profile", `application/x-protobuf; messageType="perftools.profiles.Profile"`, profile)
	err := mpw.Close()
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, apm.baseURL+api.ProfilePath, &buf)
	require.NoError(t, err)
	req.Header.Add("Content-Type", mpw.FormDataContentType())
	return testPublish(t, apm.client, req, events)
}

// testPublish exercises the publishing pipeline, from apm-server intake to beat publishing.
// It posts a payload to a running APM server via the intake API and gathers the resulting
// documents that would normally be published to Elasticsearch.
func testPublish(t *testing.T, client *http.Client, req *http.Request, events <-chan beat.Event) []byte {
	rsp, err := client.Do(req)
	require.NoError(t, err)
	got := body(t, rsp)
	assert.Equal(t, http.StatusAccepted, rsp.StatusCode, got)

	onboarded := false
	var docs []map[string]interface{}
	enc := jsonoutput.New(version.GetDefaultVersion(), jsonoutput.Config{Pretty: true})
	for _, e := range collectEvents(events, time.Second) {
		if e.Fields["processor"].(common.MapStr)["name"] == "onboarding" {
			onboarded = true
			continue
		}
		adjustMissingTimestamp(&e)
		doc, err := enc.Encode("apm-test", &e)
		require.NoError(t, err)
		var d map[string]interface{}
		err = json.Unmarshal(doc, &d)
		require.NoError(t, err)
		docs = append(docs, d)
	}
	ret, err := json.Marshal(map[string]interface{}{"events": docs})
	require.NoError(t, err)
	assert.True(t, onboarded)
	return ret
}

func TestPublishIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow tc")
	}

	for _, tc := range []struct {
		payload string
		name    string
	}{
		{payload: "errors.ndjson", name: "Errors"},
		{payload: "events.ndjson", name: "Events"},
		{payload: "metricsets.ndjson", name: "Metricsets"},
		{payload: "spans.ndjson", name: "Spans"},
		{payload: "transactions.ndjson", name: "Transactions"},
		{payload: "minimal.ndjson", name: "MinimalEvents"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// fresh APM Server for each run
			events := make(chan beat.Event)
			defer close(events)
			apm, err := setupServer(t, nil, nil, events)
			require.NoError(t, err)
			defer apm.Stop()

			b, err := loader.LoadDataAsBytes(filepath.Join("../testdata/intake-v2/", tc.payload))
			require.NoError(t, err)
			docs := testPublishIntake(t, apm, events, bytes.NewReader(b))
			approvals.AssertApproveResult(t, "test_approved_es_documents/TestPublishIntegration"+tc.name, docs)
		})
	}
}

func TestPublishIntegrationOnboarding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow tc")
	}

	events := make(chan beat.Event)
	defer close(events)
	apm, err := setupServer(t, nil, nil, events)
	require.NoError(t, err)
	defer apm.Stop()

	allEvents := collectEvents(events, time.Second)
	require.Equal(t, 1, len(allEvents))
	event := allEvents[0]
	otype, err := event.Fields.GetValue("observer.type")
	require.NoError(t, err)
	assert.Equal(t, "test-apm-server", otype.(string))
	hasListen, err := event.Fields.HasKey("observer.listening")
	require.NoError(t, err)
	assert.True(t, hasListen, "missing field: observer.listening")
}

func TestPublishIntegrationProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow tc")
	}

	for name, tc := range map[string]struct {
		metadata string
		profile  string
	}{
		"CPUProfile":         {profile: "cpu.pprof"},
		"HeapProfile":        {profile: "heap.pprof"},
		"CPUProfileMetadata": {profile: "cpu.pprof", metadata: "metadata.json"},
	} {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// fresh APM Server for each run
			events := make(chan beat.Event)
			defer close(events)
			apm, err := setupServer(t, nil, nil, events)
			require.NoError(t, err)
			defer apm.Stop()

			var metadata io.Reader
			if tc.metadata != "" {
				b, err := loader.LoadDataAsBytes(filepath.Join("../testdata/profile/", tc.metadata))
				require.NoError(t, err)
				metadata = bytes.NewReader(b)
			}
			profileBytes, err := loader.LoadDataAsBytes(filepath.Join("../testdata/profile/", tc.profile))
			require.NoError(t, err)

			docs := testPublishProfile(t, apm, events, metadata, bytes.NewReader(profileBytes))
			approvals.AssertApproveResult(t, "test_approved_es_documents/TestPublishIntegrationProfile"+name, docs,
				"profile.id", // ignore profile.id, it's randomly generated by the server
			)
		})
	}
}
