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

package firehose

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

func TestFirehoseHandler(t *testing.T) {
	for name, tc := range map[string]testcaseFirehoseHandler{
		"Success": {
			path:              "vpc_log.json",
			code:              http.StatusOK,
			id:                request.IDResponseValidAccepted,
			firehoseAccessKey: "U25jcABcd0JzTjQzUjNDemdGTHk6Ri0xMTNCdVVRdXFSR0lGYzF0Wk5Vdw==",
		},
		"failed": {
			path:              "vpc_log.json",
			code:              http.StatusBadRequest,
			id:                request.IDResponseErrorsUnauthorized,
			firehoseAccessKey: "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			// setup
			tc.setup(t)

			// call handler
			h := Handler(emptyRequestMetadata, tc.batchProcessor, tc.authenticator)
			h(tc.c)

			require.Equal(t, string(tc.id), string(tc.c.Result.ID))
			assert.Equal(t, tc.code, tc.w.Code)
			assert.Equal(t, "application/json", tc.w.Header().Get(headers.ContentType))

			if tc.code == http.StatusOK {
				assert.NotNil(t, tc.w.Body.Len())
				assert.Nil(t, tc.c.Result.Err)

				body := tc.w.Body.Bytes()
				var decoded map[string]interface{}
				err := json.Unmarshal(body, &decoded)
				assert.NoError(t, err)
				assert.Equal(t, "", decoded["errorMessage"])
				assert.Equal(t, "request-id-abcd", decoded["requestId"])
				assert.Equal(t, float64(1632865411915), decoded["timestamp"])
			} else {
				assert.NotNil(t, tc.c.Result.Err)
			}
		})
	}
}

func TestProcessFirehoseLog(t *testing.T) {
	tc := testcaseFirehoseHandler{
		path:              "vpc_log.json",
		code:              http.StatusOK,
		id:                request.IDResponseValidAccepted,
		firehoseAccessKey: "U25jcABcd0JzTjQzUjNDemdGTHk6Ri0xMTNCdVVRdXFSR0lGYzF0Wk5Vdw==",
	}

	// setup
	tc.setup(t)

	var firehose firehoseLog
	err := json.NewDecoder(tc.c.Request.Body).Decode(&firehose)
	assert.NoError(t, err)

	batch, err := processFirehoseLog(tc.c, firehose, emptyRequestMetadata)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(batch))
	assert.Equal(t, "2 123456789 eni-0b27ae2b72f7bec4c 45.146.165.96 172.31.0.75 50716 8983 6 1 40 1631651611 1631651654 REJECT OK", batch[0].Message)
}

type testcaseFirehoseHandler struct {
	c                 *request.Context
	w                 *httptest.ResponseRecorder
	r                 *http.Request
	batchProcessor    model.BatchProcessor
	authenticator     *auth.Authenticator
	path              string
	firehoseAccessKey string

	code int
	id   request.ResultID
}

func (tc *testcaseFirehoseHandler) setup(t *testing.T) {
	if tc.batchProcessor == nil {
		tc.batchProcessor = modelprocessor.Nop{}
	}

	authenticator, err := auth.NewAuthenticator(config.AgentAuth{})
	require.NoError(t, err)
	tc.authenticator = authenticator

	if tc.r == nil {
		data, err := ioutil.ReadFile(filepath.Join("../../../testdata/firehose", tc.path))
		require.NoError(t, err)

		tc.r = httptest.NewRequest("POST", "/", bytes.NewBuffer(data))
		tc.r.Header.Add("Content-Type", "application/json")
		tc.r.Header.Add("X-Amz-Firehose-Source-Arn", "arn:aws:firehose:us-east-1:123456789:deliverystream/vpc-flow-log-stream-http-endpoint")
		if tc.firehoseAccessKey != "" {
			tc.r.Header.Add("X-Amz-Firehose-Access-Key", tc.firehoseAccessKey)
		}
	}

	q := tc.r.URL.Query()
	q.Add("verbose", "")
	tc.r.URL.RawQuery = q.Encode()
	tc.r.Header.Add("Accept", "application/json")

	tc.w = httptest.NewRecorder()
	tc.c = request.NewContext()
	tc.c.Reset(tc.w, tc.r)
}

func emptyRequestMetadata(*request.Context) model.APMEvent {
	return model.APMEvent{}
}
