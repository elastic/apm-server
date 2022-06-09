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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/api"
)

const timestampFormat = "2006-01-02T15:04:05.000Z07:00"

var timestampOverride = time.Date(2019, 1, 9, 21, 40, 53, 690, time.UTC)

// adjustMissingTimestamp sets @timestamp and timestamp.us to known values for events that originally omitted a ts
func adjustMissingTimestamp(doc []byte) []byte {
	timestampField := gjson.GetBytes(doc, "@timestamp")
	timestamp, err := time.Parse(timestampFormat, timestampField.String())
	if err != nil {
		panic(err)
	}
	if time.Since(timestamp) < 5*time.Minute {
		var err error
		doc, err = sjson.SetBytes(doc, "@timestamp", timestampOverride.Format(timestampFormat))
		if err != nil {
			panic(err)
		}
		if gjson.GetBytes(doc, "processor.name").String() != "metric" {
			doc, err = sjson.SetBytes(doc, "timestamp.us", timestampOverride.UnixNano()/1000)
			if err != nil {
				panic(err)
			}
		}
	}
	return doc
}

// testPublishIntake exercises the publishing pipeline, from apm-server intake to beat publishing.
// It posts a payload to a running APM server via the intake API and gathers the resulting documents that would
// normally be published to Elasticsearch.
func testPublishIntake(t *testing.T, apm *testBeater, docs <-chan []byte, payload io.Reader) [][]byte {
	req, err := http.NewRequest(http.MethodPost, apm.baseURL+api.IntakePath, payload)
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-ndjson")
	return testPublish(t, apm.client, req, docs)
}

func testPublishProfile(t *testing.T, apm *testBeater, docs <-chan []byte, metadata, profile io.Reader) [][]byte {
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
	return testPublish(t, apm.client, req, docs)
}

// testPublish exercises the publishing pipeline, from apm-server intake to beat publishing.
// It posts a payload to a running APM server via the intake API and gathers the resulting
// documents that would normally be published to Elasticsearch.
func testPublish(t *testing.T, client *http.Client, req *http.Request, docsChan <-chan []byte) [][]byte {
	// The request may block until events are published, so we receive them in the background.
	acceptedCh := make(chan int, 1)
	var docs [][]byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		var accepted int
		for accepted == 0 || len(docs) != accepted {
			select {
			case n := <-acceptedCh:
				accepted = n
			case doc := <-docsChan:
				doc = adjustMissingTimestamp(doc)
				docs = append(docs, doc)
			}
		}
	}()

	req.URL.RawQuery = "verbose=true"
	rsp, err := client.Do(req)
	require.NoError(t, err)
	got := body(t, rsp)
	assert.Equal(t, http.StatusAccepted, rsp.StatusCode, got)

	var result struct {
		Accepted int
	}
	err = json.Unmarshal([]byte(got), &result)
	require.NoError(t, err)
	acceptedCh <- result.Accepted

	<-done
	return docs
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
		{payload: "transactions-huge_traces.ndjson", name: "TransactionsHugeTraces"},
		{payload: "minimal.ndjson", name: "MinimalEvents"},
		{payload: "unknown-span-type.ndjson", name: "UnknownSpanType"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// fresh APM Server for each run
			docsChan := make(chan []byte)
			apm, err := setupServer(t, nil, nil, docsChan)
			require.NoError(t, err)
			defer apm.Stop()

			b, err := os.ReadFile(filepath.Join("../testdata/intake-v2", tc.payload))
			require.NoError(t, err)
			docs := testPublishIntake(t, apm, docsChan, bytes.NewReader(b))
			approvaltest.ApproveEventDocs(t, "test_approved_es_documents/TestPublishIntegration"+tc.name, docs)
		})
	}
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
			// fresh APM Server for each run
			docsChan := make(chan []byte)
			apm, err := setupServer(t, nil, nil, docsChan)
			require.NoError(t, err)
			defer apm.Stop()

			var metadata io.Reader
			if tc.metadata != "" {
				b, err := os.ReadFile(filepath.Join("../testdata/profile", tc.metadata))
				require.NoError(t, err)
				metadata = bytes.NewReader(b)
			}
			profileBytes, err := os.ReadFile(filepath.Join("../testdata/profile", tc.profile))
			require.NoError(t, err)

			docs := testPublishProfile(t, apm, docsChan, metadata, bytes.NewReader(profileBytes))
			approvaltest.ApproveEventDocs(t, "test_approved_es_documents/TestPublishIntegrationProfile"+name, docs,
				"profile.id", // ignore profile.id, it's randomly generated by the server
			)
		})
	}
}
