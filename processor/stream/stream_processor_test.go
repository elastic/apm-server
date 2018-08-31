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

package stream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"testing/iotest"
	"time"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"

	"github.com/elastic/apm-server/publish"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/transform"
)

func validMetadata() string {
	return `{"metadata": {"service": {"name": "myservice", "agent": {"name": "test", "version": "1.0"}}}}`
}

func approveResult(t *testing.T, actualResponse *Result, name string) {
	resultName := fmt.Sprintf("approved-stream-result/testIntegrationResult%s", name)
	resultJSON, err := json.Marshal(actualResponse)
	require.NoError(t, err)

	var resultmap map[string]interface{}
	err = json.Unmarshal(resultJSON, &resultmap)
	require.NoError(t, err)

	verifyErr := tests.ApproveJson(resultmap, resultName, map[string]string{})
	if verifyErr != nil {
		assert.Fail(t, fmt.Sprintf("Test %s failed with error: %s", name, verifyErr.Error()))
	}
}

func TestV2HandlerReadStreamError(t *testing.T) {
	var transformables []transform.Transformable
	report := func(ctx context.Context, p publish.PendingReq) error {
		transformables = append(transformables, p.Transformables...)
		return nil
	}
	b, err := loader.LoadDataAsBytes("../testdata/intake-v2/transactions.ndjson")
	require.NoError(t, err)
	bodyReader := bytes.NewBuffer(b)
	timeoutReader := iotest.TimeoutReader(bodyReader)

	reader := decoder.NewNDJSONStreamReader(timeoutReader)
	sp := StreamProcessor{}

	actualResult := sp.HandleStream(context.Background(), map[string]interface{}{}, reader, report)
	approveResult(t, actualResult, "ReadError")
}

func TestV2HandlerReportingStreamError(t *testing.T) {
	for _, test := range []struct {
		name   string
		report func(ctx context.Context, p publish.PendingReq) error
	}{
		{
			name: "ShuttingDown",
			report: func(ctx context.Context, p publish.PendingReq) error {
				return publish.ErrChannelClosed
			},
		}, {
			name: "QueueFull",
			report: func(ctx context.Context, p publish.PendingReq) error {
				return publish.ErrFull
			},
		},
	} {

		b, err := loader.LoadDataAsBytes("../testdata/intake-v2/transactions.ndjson")
		require.NoError(t, err)
		bodyReader := bytes.NewBuffer(b)

		reader := decoder.NewNDJSONStreamReader(bodyReader)

		sp := StreamProcessor{}
		actualResult := sp.HandleStream(context.Background(), map[string]interface{}{}, reader, test.report)
		approveResult(t, actualResult, test.name)
	}
}

func TestIntegration(t *testing.T) {
	report := func(ctx context.Context, p publish.PendingReq) error {
		var events []beat.Event
		for _, transformable := range p.Transformables {
			events = append(events, transformable.Transform(p.Tcontext)...)
		}
		name := ctx.Value("name").(string)
		verifyErr := tests.ApproveEvents(events, name, nil)
		if verifyErr != nil {
			assert.Fail(t, fmt.Sprintf("Test %s failed with error: %s", name, verifyErr.Error()))
		}
		return nil
	}

	for _, test := range []struct {
		path string
		name string
	}{
		{path: "../testdata/intake-v2/errors.ndjson", name: "Errors"},
		{path: "../testdata/intake-v2/transactions.ndjson", name: "Transactions"},
		{path: "../testdata/intake-v2/spans.ndjson", name: "Spans"},
		{path: "../testdata/intake-v2/metrics.ndjson", name: "Metrics"},
		{path: "../testdata/intake-v2/minimal_process.ndjson", name: "MixedMinimalProcess"},
		{path: "../testdata/intake-v2/minimal_service.ndjson", name: "MinimalService"},
		{path: "../testdata/intake-v2/metadata_null_values.ndjson", name: "MetadataNullValues"},
		{path: "../testdata/intake-v2/invalid-event.ndjson", name: "InvalidEvent"},
		{path: "../testdata/intake-v2/invalid-json-event.ndjson", name: "InvalidJSONEvent"},
		{path: "../testdata/intake-v2/invalid-json-metadata.ndjson", name: "InvalidJSONMetadata"},
		{path: "../testdata/intake-v2/invalid-metadata.ndjson", name: "InvalidMetadata"},
		{path: "../testdata/intake-v2/invalid-metadata-2.ndjson", name: "InvalidMetadata2"},
		{path: "../testdata/intake-v2/unrecognized-event.ndjson", name: "UnrecognizedEvent"},
		{path: "../testdata/intake-v2/optional-timestamps.ndjson", name: "OptionalTimestamps"},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, err := loader.LoadDataAsBytes(test.path)
			require.NoError(t, err)
			bodyReader := bytes.NewBuffer(b)

			name := fmt.Sprintf("approved-es-documents/testV2IntakeIntegration%s", test.name)
			ctx := context.WithValue(context.Background(), "name", name)
			reqTimestamp, err := time.Parse(time.RFC3339, "2018-08-01T10:00:00Z")
			ctx = utility.ContextWithRequestTime(ctx, reqTimestamp)

			reader := decoder.NewNDJSONStreamReader(bodyReader)

			reqDecoderMeta := map[string]interface{}{
				"system": map[string]interface{}{
					"ip": "192.0.0.1",
				},
			}

			actualResult := (&StreamProcessor{}).HandleStream(ctx, reqDecoderMeta, reader, report)
			approveResult(t, actualResult, test.name)
		})
	}
}
