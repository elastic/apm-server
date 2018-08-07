package beater

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

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

	for idx, test := range []struct {
		body         string
		contentType  string
		err          *streamResponse
		expectedCode int
	}{
		{
			body:        "",
			contentType: "",
			err: &streamResponse{
				Errors: map[int]map[string]int{
					400: map[string]int{
						"data decoding error: invalid content type: ''": 1,
					},
				},
				Accepted: -1,
				Dropped:  -1,
				Invalid:  -1,
			},
			expectedCode: 400,
		},
		{
			body: strings.Join([]string{
				`{"metadata": {}}`,
				`{"span": {}}`,
			}, "\n"),
			contentType:  "application/x-ndjson",
			expectedCode: 400,
			err: &streamResponse{
				Errors: map[int]map[string]int{
					400: map[string]int{
						"data validation error: Problem validating JSON document against schema: I[#] S[#] doesn't validate with \"metadata#\"\n  I[#] S[#/required] missing properties: \"service\"": 1,
					},
				},
				Dropped: 1,
			},
		},
		// {
		// 	body: strings.Join([]string{
		// 		validMetadata(),
		// 		`{"transaction": {"name": "asdasd"}}`,
		// 	}, "\n"),
		// 	contentType:  "application/x-ndjson",
		// 	expectedCode: 400,
		// 	err: &streamResponse{
		// 		Errors: map[int]map[string]int{
		// 			400: map[string]int{
		// 				"data validation error: Problem validating JSON document against schema: invalid jsonType: common.MapStr": 1,
		// 			},
		// 		},
		// 		Dropped: 1,
		// 	},
		// },
	} {
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
	}

}
