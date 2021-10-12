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
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

const (
	testARN           = "arn:aws:firehose:us-east-1:123456789:deliverystream/vpc-flow-log-stream-http-endpoint"
	expectedPartition = "aws"
	expectedService   = "firehose"
	expectedAccountID = "123456789"
	expectedRegion    = "us-east-1"
	expectedResource  = "deliverystream/vpc-flow-log-stream-http-endpoint"
	expectedMessage   = "2 123456789 eni-0b27ae2b72f7bec4c 45.146.165.96 172.31.0.75 50716 8983 6 1 40 1631651611 1631651654 REJECT OK"
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
			code:              http.StatusUnauthorized,
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
	var batches []model.Batch
	tc := testcaseFirehoseHandler{
		path:              "vpc_log.json",
		code:              http.StatusOK,
		id:                request.IDResponseValidAccepted,
		firehoseAccessKey: "U25jcABcd0JzTjQzUjNDemdGTHk6Ri0xMTNCdVVRdXFSR0lGYzF0Wk5Vdw==",
		batchProcessor: model.ProcessBatchFunc(func(ctx context.Context, batch *model.Batch) error {
			batches = append(batches, *batch)
			return nil
		}),
	}

	tc.setup(t)
	h := Handler(tc.batchProcessor, tc.authenticator)
	h(tc.c)

	require.Len(t, batches, 1)
	require.Len(t, batches[0], 1)
	event := batches[0][0]

	assert.Equal(t, expectedMessage, event.Message)
	assert.Equal(t, expectedRegion, event.Cloud.Origin.Region)
	assert.Equal(t, expectedAccountID, event.Cloud.Origin.AccountID)
	assert.Equal(t, testARN, event.Service.Origin.ID)
	assert.Equal(t, expectedResource, event.Service.Origin.Name)
}

func TestAuth(t *testing.T) {
	tc := testcaseFirehoseHandler{
		path:              "vpc_log.json",
		code:              http.StatusOK,
		id:                request.IDResponseValidAccepted,
		firehoseAccessKey: "U25jcABcd0JzTjQzUjNDemdGTHk6Ri0xMTNCdVVRdXFSR0lGYzF0Wk5Vdw==",
	}
	var authzCalled bool
	tc.authenticator = authenticatorFunc(func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
		var authz authorizerFunc = func(ctx context.Context, action auth.Action, resource auth.Resource) error {
			authzCalled = true
			return nil
		}
		require.Equal(t, "ApiKey", kind)
		require.Equal(t, tc.firehoseAccessKey, token)
		return auth.AuthenticationDetails{Method: auth.MethodAPIKey}, authz, nil
	})
	tc.batchProcessor = model.ProcessBatchFunc(func(ctx context.Context, batch *model.Batch) error {
		return auth.Authorize(ctx, auth.ActionEventIngest, auth.Resource{})
	})
	tc.setup(t)
	h := Handler(tc.batchProcessor, tc.authenticator)
	h(tc.c)

	require.Equal(t, string(tc.id), string(tc.c.Result.ID))
	assert.Equal(t, tc.code, tc.w.Code)
	assert.True(t, authzCalled)
}

func TestAuthError(t *testing.T) {
	tc := testcaseFirehoseHandler{
		path:              "vpc_log.json",
		code:              http.StatusUnauthorized,
		id:                request.IDResponseErrorsUnauthorized,
		firehoseAccessKey: "invalid",
	}
	tc.authenticator = authenticatorFunc(func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
		return auth.AuthenticationDetails{}, nil, errors.New("authentication failed")
	})
	tc.setup(t)
	h := Handler(tc.batchProcessor, tc.authenticator)
	h(tc.c)
	require.Equal(t, string(tc.id), string(tc.c.Result.ID))
	assert.Equal(t, tc.code, tc.w.Code)
}

type testcaseFirehoseHandler struct {
	c                 *request.Context
	w                 *httptest.ResponseRecorder
	r                 *http.Request
	batchProcessor    model.BatchProcessor
	authenticator     Authenticator
	path              string
	firehoseAccessKey string

	code int
	id   request.ResultID
}

func (tc *testcaseFirehoseHandler) setup(t *testing.T) {
	if tc.batchProcessor == nil {
		tc.batchProcessor = modelprocessor.Nop{}
	}

	if tc.authenticator == nil {
		authenticator, err := auth.NewAuthenticator(config.AgentAuth{})
		require.NoError(t, err)
		tc.authenticator = authenticator
	}

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
	assert.Equal(t, expectedPartition, arnParsed.Partition)
	assert.Equal(t, expectedService, arnParsed.Service)
	assert.Equal(t, expectedAccountID, arnParsed.AccountID)
	assert.Equal(t, expectedRegion, arnParsed.Region)
	assert.Equal(t, expectedResource, arnParsed.Resource)
}

type authenticatorFunc func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error)

func (f authenticatorFunc) Authenticate(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
	return f(ctx, kind, token)
}

type authorizerFunc func(ctx context.Context, action auth.Action, resource auth.Resource) error

func (f authorizerFunc) Authorize(ctx context.Context, action auth.Action, resource auth.Resource) error {
	return f(ctx, action, resource)
}
