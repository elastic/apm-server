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

package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"runtime/pprof"
	"strings"
	"testing"

	"github.com/elastic/apm-server/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/publish"
)

const pprofContentType = `application/x-protobuf; messageType="perftools.profiles.Profile"`

func TestHandler(t *testing.T) {
	for name, tc := range map[string]testcaseIntakeHandler{
		"MethodNotAllowed": {
			r:  httptest.NewRequest(http.MethodGet, "/", nil),
			id: request.IDResponseErrorsMethodNotAllowed,
		},
		"RequestInvalidContentType": {
			r: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", nil)
				req.Header.Set(headers.ContentType, "text/plain")
				return req
			}(),
			id: request.IDResponseErrorsValidate,
		},
		"Closing": {
			batchProcessor: func(t *testing.T) model.BatchProcessor {
				return model.ProcessBatchFunc(func(context.Context, *model.Batch) error {
					return publish.ErrChannelClosed
				})
			},
			id: request.IDResponseErrorsShuttingDown,
		},
		"FullQueue": {
			batchProcessor: func(t *testing.T) model.BatchProcessor {
				return model.ProcessBatchFunc(func(context.Context, *model.Batch) error {
					return publish.ErrFull
				})
			},
			id: request.IDResponseErrorsFullQueue,
		},
		"Empty": {
			id:   request.IDResponseValidAccepted,
			body: prettyJSON(map[string]interface{}{"accepted": 0}),
		},
		"UnknownPartIgnored": {
			id:   request.IDResponseValidAccepted,
			body: prettyJSON(map[string]interface{}{"accepted": 0}),
			parts: []part{{
				name:        "foo",
				contentType: "text/plain",
				body:        strings.NewReader(""),
			}},
		},

		"MetadataTooLarge": {
			id: request.IDResponseErrorsRequestTooLarge,
			parts: []part{{
				name:        "metadata",
				contentType: "application/json",
				body:        strings.NewReader("{" + strings.Repeat(" ", 10*1024) + "}"),
			}},
		},
		"MetadataInvalidContentType": {
			id: request.IDResponseErrorsValidate,
			parts: []part{{
				name:        "metadata",
				contentType: "text/plain",
				body:        strings.NewReader(`{"service":{"name":"foo","agent":{}}}`),
			}},
		},
		"MetadataInvalidJSON": {
			id: request.IDResponseErrorsDecode,
			parts: []part{{
				name:        "metadata",
				contentType: "application/json",
				body:        strings.NewReader("{..."),
			}},
		},
		"MetadataInvalid": {
			id: request.IDResponseErrorsValidate,
			parts: []part{{
				name:        "metadata",
				contentType: "application/json",
				body:        strings.NewReader("{}"), // does not validate
			}},
		},

		"Profile": {
			id: request.IDResponseValidAccepted,
			parts: []part{
				heapProfilePart(),
				{
					name: "profile",
					// No messageType param specified, so pprof is assumed.
					contentType: "application/x-protobuf",
					body:        heapProfileBody(),
				},
				{
					name:        "metadata",
					contentType: "application/json",
					body:        strings.NewReader(`{"service":{"name":"foo","agent":{"name":"java","version":"1.2.0"}}}`),
				},
			},
			body:    prettyJSON(map[string]interface{}{"accepted": 2}),
			reports: 1,
			batchProcessor: func(t *testing.T) model.BatchProcessor {
				return model.ProcessBatchFunc(func(ctx context.Context, batch *model.Batch) error {
					require.Len(t, *batch, 2)
					for _, event := range *batch {
						assert.Equal(t, "foo", event.Profile.Metadata.Service.Name)
					}
					return nil
				})
			},
		},
		"ProfileInvalidContentType": {
			id: request.IDResponseErrorsValidate,
			parts: []part{{
				name:        "profile",
				contentType: "text/plain",
				body:        strings.NewReader(""),
			}},
			body: prettyJSON(map[string]interface{}{"accepted": 0}),
		},
		"ProfileInvalidMessageType": {
			id: request.IDResponseErrorsValidate,
			parts: []part{{
				name: "profile",
				// Improperly formatted "messageType" param
				// in Content-Type from APM Agent Go v1.6.0.
				contentType: "application/x-protobuf; messageType=”perftools.profiles.Profile",
				body:        strings.NewReader(""),
			}},
			body: prettyJSON(map[string]interface{}{"accepted": 0}),
		},
		"ProfileInvalid": {
			id: request.IDResponseErrorsDecode,
			parts: []part{{
				name:        "profile",
				contentType: pprofContentType,
				body:        strings.NewReader("foo"),
			}},
			body: prettyJSON(map[string]interface{}{"accepted": 0}),
		},
		"ProfileTooLarge": {
			id: request.IDResponseErrorsRequestTooLarge,
			parts: []part{
				heapProfilePart(),
				part{
					name:        "profile",
					contentType: pprofContentType,
					body:        strings.NewReader(strings.Repeat("*", 10*1024*1024)),
				},
			},
			body: prettyJSON(map[string]interface{}{"accepted": 0}),
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.setup(t)
			Handler(emptyRequestMetadata, tc.batchProcessor(t))(tc.c)

			assert.Equal(t, string(tc.id), string(tc.c.Result.ID))
			resultStatus := request.MapResultIDToStatus[tc.id]
			assert.Equal(t, resultStatus.Code, tc.w.Code)
			assert.Equal(t, "application/json", tc.w.Header().Get(headers.ContentType))

			assert.Zero(t, tc.reports)
			if tc.id == request.IDResponseValidAccepted {
				assert.Equal(t, tc.body, tc.w.Body.String())
				assert.Nil(t, tc.c.Result.Err)
			} else {
				assert.NotNil(t, tc.c.Result.Err)
				assert.NotZero(t, tc.w.Body.Len())
			}
		})
	}
}

type testcaseIntakeHandler struct {
	c              *request.Context
	w              *httptest.ResponseRecorder
	r              *http.Request
	batchProcessor func(t *testing.T) model.BatchProcessor
	reports        int
	parts          []part

	id   request.ResultID
	body string
}

func (tc *testcaseIntakeHandler) setup(t *testing.T) {
	if tc.batchProcessor == nil {
		tc.batchProcessor = func(t *testing.T) model.BatchProcessor {
			return model.ProcessBatchFunc(func(context.Context, *model.Batch) error { return nil })
		}
	}
	if tc.reports > 0 {
		orig := tc.batchProcessor
		tc.batchProcessor = func(t *testing.T) model.BatchProcessor {
			orig := orig(t)
			return model.ProcessBatchFunc(func(ctx context.Context, batch *model.Batch) error {
				tc.reports--
				return orig.ProcessBatch(ctx, batch)
			})
		}
	}
	if tc.r == nil {
		var buf bytes.Buffer
		mpw := multipart.NewWriter(&buf)
		for _, part := range tc.parts {
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q`, part.name))
			h.Set("Content-Type", part.contentType)

			p, err := mpw.CreatePart(h)
			require.NoError(t, err)
			_, err = io.Copy(p, part.body)
			require.NoError(t, err)
		}
		mpw.Close()

		tc.r = httptest.NewRequest(http.MethodPost, "/", &buf)
		tc.r.Header.Set("Content-Type", mpw.FormDataContentType())
	}
	tc.r.Header.Add("Accept", "application/json")
	tc.w = httptest.NewRecorder()
	tc.c = request.NewContext()
	tc.c.Reset(tc.w, tc.r)
}

func heapProfilePart() part {
	return part{name: "profile", contentType: pprofContentType, body: heapProfileBody()}
}

func heapProfileBody() io.Reader {
	var buf bytes.Buffer
	if err := pprof.WriteHeapProfile(&buf); err != nil {
		panic(err)
	}
	return &buf
}

type part struct {
	name        string
	contentType string
	body        io.Reader
}

func prettyJSON(v interface{}) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.Encode(v)
	return buf.String()
}

func emptyRequestMetadata(*request.Context) model.Metadata {
	return model.Metadata{}
}
