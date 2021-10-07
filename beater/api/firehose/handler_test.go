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
	"github.com/elastic/apm-server/processor/stream"
)

const (
	testARN = "arn:aws:firehose:us-east-1:123456789:deliverystream/vpc-flow-log-stream-http-endpoint"
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
			h := Handler(tc.batchProcessor, tc.authenticator)
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

	batch, err := processFirehoseLog(tc.c, firehose)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(batch))
	assert.Equal(t, "123456789", batch[0].Cloud.Origin.AccountID)
	assert.Equal(t, "us-east-1", batch[0].Cloud.Origin.Region)
	assert.Equal(t, "deliverystream/vpc-flow-log-stream-http-endpoint", batch[0].Service.Origin.Name)
	assert.Equal(t, testARN, batch[0].Service.Origin.ID)
	assert.Equal(t, "logs", batch[0].DataStream.Type)
	assert.Equal(t, "firehose", batch[0].DataStream.Dataset)
}

type testcaseFirehoseHandler struct {
	c                 *request.Context
	w                 *httptest.ResponseRecorder
	r                 *http.Request
	processor         *stream.Processor
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
		tc.r.Header.Add("X-Amz-Firehose-Source-Arn", testARN)
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

func TestParseARN(t *testing.T) {
	arnParsed := parseARN(testARN)
	assert.Equal(t, "aws", arnParsed.Partition)
	assert.Equal(t, "firehose", arnParsed.Service)
	assert.Equal(t, "123456789", arnParsed.AccountID)
	assert.Equal(t, "us-east-1", arnParsed.Region)
	assert.Equal(t, "deliverystream/vpc-flow-log-stream-http-endpoint", arnParsed.Resource)
}
