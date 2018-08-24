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
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elastic/apm-server/utility"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/beats/libbeat/beat"
)

func TestV2IntakeIntegration(t *testing.T) {
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

	c := defaultConfig("7.0.0")

	handler := (&v2BackendRoute).Handler(c, report)

	for _, test := range []struct {
		path   string
		name   string
		status int
	}{
		{status: 202, path: "../testdata/intake-v2/errors.ndjson", name: "Errors"},
		{status: 202, path: "../testdata/intake-v2/transactions.ndjson", name: "Transactions"},
		{status: 202, path: "../testdata/intake-v2/spans.ndjson", name: "Spans"},
		{status: 202, path: "../testdata/intake-v2/metrics.ndjson", name: "Metrics"},
		{status: 202, path: "../testdata/intake-v2/minimal_process.ndjson", name: "MixedMinimalProcess"},
		{status: 202, path: "../testdata/intake-v2/minimal_service.ndjson", name: "MinimalService"},
		{status: 202, path: "../testdata/intake-v2/metadata_null_values.ndjson", name: "MetadataNullValues"},
		{status: 400, path: "../testdata/intake-v2/invalid-event.ndjson", name: "InvalidEvent"},
	} {

		b, err := loader.LoadDataAsBytes(test.path)
		require.NoError(t, err)
		bodyReader := bytes.NewBufferString(string(b))

		r := httptest.NewRequest("POST", "/v2/intake", bodyReader)
		r.Header.Add("Content-Type", "application/x-ndjson")
		r.Header.Add("X-Real-Ip", "192.0.0.1")

		w := httptest.NewRecorder()

		name := fmt.Sprintf("approved-es-documents/testV2IntakeIntegration%s", test.name)
		r = r.WithContext(context.WithValue(r.Context(), "name", name))
		reqTimestamp, err := time.Parse(time.RFC3339, "2018-08-01T10:00:00Z")
		r = r.WithContext(utility.ContextWithRequestTime(r.Context(), reqTimestamp))
		handler.ServeHTTP(w, r)

		assert.Equal(t, test.status, w.Code)

	}
}
