package beater

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elastic/apm-server/model/metric"
	"github.com/elastic/apm-server/model/transaction"

	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/transform"
	"github.com/stretchr/testify/assert"
)

func validMetadata() string {
	return `{"metadata": {"service": {"name": "myservice", "agent": {"name": "test", "version": "1.0"}}}}`
}

func TestV2Handler(t *testing.T) {
	var transformables []transform.Transformable
	report := func(ctx context.Context, p pendingReq) error {
		transformables = append(transformables, p.transformables...)
		return nil
	}

	c := defaultConfig("7.0.0")

	handler := (&v2Route{backendRouteType}).Handler(c, report)

	tx1 := "tx1"
	timestamp, err := time.Parse(time.RFC3339, "2018-01-01T10:00:00Z")
	assert.NoError(t, err)

	for idx, test := range []struct {
		body         string
		contentType  string
		err          *streamResponse
		expectedCode int
		reported     []transform.Transformable
	}{
		{
			body:        "",
			contentType: "",
			err: &streamResponse{
				Errors: map[streamErrorType]errorDetails{
					"ERR_CONTENT_TYPE": errorDetails{
						Count:   1,
						Message: "invalid content-type. Expected 'application/x-ndjson'",
					},
				},
				Accepted: -1,
				Dropped:  -1,
				Invalid:  1,
			},
			expectedCode: 400,
			reported:     []transform.Transformable{},
		},
		{
			body: strings.Join([]string{
				`{"metadata": {}}`,
				`{"span": {}}`,
			}, "\n"),
			contentType:  "application/x-ndjson",
			expectedCode: 400,
			err: &streamResponse{
				Errors: map[streamErrorType]errorDetails{
					"ERR_SCHEMA_VALIDATION": errorDetails{
						Count:   1,
						Message: "validation error",
						Documents: []*ValidationError{
							{
								Error:          "Problem validating JSON document against schema: I[#] S[#] doesn't validate with \"metadata#\"\n  I[#] S[#/required] missing properties: \"service\"",
								OffendingEvent: "{\"metadata\": {}}\n",
							},
						},
					},
				},
				Dropped: 1,
			},
			reported: []transform.Transformable{},
		},
		{
			body: strings.Join([]string{
				validMetadata(),
				`{"transaction": {"name": "tx1", "id": "8ace3f94-cd01-462c-b069-57dc28ebdfc8", "duration": 12, "type": "request", "timestamp": "2018-01-01T10:00:00Z"}}`,
				`{"span": {"name": "sp1", "duration": 20, "start": 10, "type": "db"}}`,
				`{"metric": {"samples": {"my-metric": {"value": 99}}, "timestamp": "2018-01-01T10:00:00Z"}}`,
			}, "\n"),
			contentType:  "application/x-ndjson",
			expectedCode: http.StatusAccepted,
			reported: []transform.Transformable{
				&transaction.Event{Name: &tx1, Id: "8ace3f94-cd01-462c-b069-57dc28ebdfc8", Duration: 12, Type: "request", Spans: []*span.Span{}, Timestamp: timestamp},
				&span.Span{Name: "sp1", Duration: 20.0, Start: 10, Type: "db"},
				&metric.Metric{Samples: []*metric.Sample{&metric.Sample{Name: "my-metric", Value: 99}}, Timestamp: timestamp},
			},
		},
	} {
		transformables = []transform.Transformable{}
		bodyReader := bytes.NewBufferString(test.body)

		r := httptest.NewRequest("POST", "/v2/intake", bodyReader)
		r.Header.Add("Content-Type", test.contentType)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		assert.Equal(t, test.expectedCode, w.Code, "Failed at index %d: %s", idx, w.Body.String())
		if test.err != nil {
			var actualResponse streamResponse
			assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &actualResponse))
			assert.Equal(t, *test.err, actualResponse, "Failed at index %d", idx)
		}

		assert.Equal(t, test.reported, transformables)

	}

}
