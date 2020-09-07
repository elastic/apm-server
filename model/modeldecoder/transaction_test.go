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

package modeldecoder

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/tests"
)

var (
	trID      = "123"
	trType    = "type"
	trName    = "foo()"
	trResult  = "555"
	trOutcome = "success"

	trDuration = 6.0

	traceID  = "0147258369012345abcdef0123456789"
	parentID = "abcdef0123456789"

	timestampParsed = time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)
	timestampEpoch  = json.Number(fmt.Sprintf("%d", timestampParsed.UnixNano()/1000))

	marks = map[string]interface{}{"navigationTiming": map[string]interface{}{
		"appBeforeBootstrap": 608.9300000000001,
		"navigationStart":    -21.0,
	}}

	trUserName  = "jane"
	trUserID    = "abc123"
	trUserEmail = "j@d.com"
	trUserIP    = "127.0.0.1"

	trPageURL                = "https://mypage.com"
	trPageReferer            = "http:mypage.com"
	trRequestHeaderUserAgent = "go-1.1"
)

var fullTransactionInput = map[string]interface{}{
	"id":         trID,
	"type":       trType,
	"name":       trName,
	"duration":   trDuration,
	"timestamp":  timestampEpoch,
	"result":     trResult,
	"outcome":    trOutcome,
	"sampled":    true,
	"trace_id":   traceID,
	"parent_id":  parentID,
	"span_count": map[string]interface{}{"dropped": 12.0, "started": 6.0},
	"marks":      marks,
	"context": map[string]interface{}{
		"a":      "b",
		"custom": map[string]interface{}{"abc": 1},
		"user":   map[string]interface{}{"username": trUserName, "email": trUserEmail, "ip": trUserIP, "id": trUserID},
		"tags":   map[string]interface{}{"foo": "bar"},
		"page":   map[string]interface{}{"url": trPageURL, "referer": trPageReferer},
		"request": map[string]interface{}{
			"method":  "POST",
			"url":     map[string]interface{}{"raw": "127.0.0.1"},
			"headers": map[string]interface{}{"user-agent": trRequestHeaderUserAgent},
		},
		"response": map[string]interface{}{
			"finished": false,
			"headers":  map[string]interface{}{"Content-Type": "text/html"},
		},
	},
	"experience": map[string]interface{}{
		"cls":     1,
		"fid":     2,
		"tbt":     3,
		"ignored": 4,
	},
}

func TestDecodeTransactionInvalid(t *testing.T) {
	err := DecodeTransaction(Input{Raw: nil}, &model.Batch{})
	require.EqualError(t, err, "failed to validate transaction: error validating JSON: input missing")

	err = DecodeTransaction(Input{Raw: ""}, &model.Batch{})
	require.EqualError(t, err, "failed to validate transaction: error validating JSON: invalid input type")

	baseInput := map[string]interface{}{
		"type":       "type",
		"trace_id":   "trace_id",
		"id":         "id",
		"duration":   123,
		"span_count": map[string]interface{}{"dropped": 1.0, "started": 2.0},
	}

	for name, test := range map[string]struct {
		input map[string]interface{}
		err   string
	}{
		"missing trace_id": {
			input: map[string]interface{}{"trace_id": nil},
			err:   "missing properties: \"trace_id\"",
		},
		"negative duration": {
			input: map[string]interface{}{"duration": -1.0},
			err:   "duration.*must be >= 0 but found -1",
		},
		"invalid outcome": {
			input: map[string]interface{}{"outcome": `¯\_(ツ)_/¯`},
			err:   `outcome.*must be one of <nil>, "success", "failure", "unknown"`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := make(map[string]interface{})
			for k, v := range baseInput {
				input[k] = v
			}
			for k, v := range test.input {
				if v == nil {
					delete(input, k)
				} else {
					input[k] = v
				}
			}
			err := DecodeTransaction(Input{Raw: input}, &model.Batch{})
			assert.Error(t, err)
			assert.Regexp(t, test.err, err.Error())
		})
	}
}

func TestTransactionDecodeRUMV3Marks(t *testing.T) {
	// TODO use DecodeRUMV3Transaction to ensure we test with completely valid input data.

	// unknown fields are ignored
	input := map[string]interface{}{
		"foo": 0,
		"a": map[string]interface{}{
			"foo": 0,
			"dc":  1.2,
		},
		"nt": map[string]interface{}{
			"foo": 0,
			"dc":  1.2,
		},
	}
	marks := decodeRUMV3Marks(input, Config{HasShortFieldNames: true})

	var f = 1.2
	assert.Equal(t, model.TransactionMarks{
		"agent":            {"domComplete": f},
		"navigationTiming": {"domComplete": f},
	}, marks)
}

func TestTransactionEventDecode(t *testing.T) {
	id, trType, name, result := "123", "type", "foo()", "555"
	outcome := "success"
	requestTime := time.Now()
	timestampParsed := time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)
	timestampEpoch := json.Number(fmt.Sprintf("%d", timestampParsed.UnixNano()/1000))

	traceID, parentID := "0147258369012345abcdef0123456789", "abcdef0123456789"
	dropped, started, duration := 12, 6, 1.67
	name, userID, email, userIP := "jane", "abc123", "j@d.com", "127.0.0.1"
	url, referer, origURL := "https://mypage.com", "http:mypage.com", "127.0.0.1"
	sampled := true
	labels := model.Labels{"foo": "bar"}
	ua := "go-1.1"
	page := model.Page{URL: model.ParseURL(url, ""), Referer: &referer}
	request := model.Req{Method: "post", Socket: &model.Socket{}, Headers: http.Header{"User-Agent": []string{ua}}}
	response := model.Resp{Finished: new(bool), MinimalResp: model.MinimalResp{Headers: http.Header{"Content-Type": []string{"text/html"}}}}
	badRequestResp, internalErrorResp := 400, 500
	h := model.Http{Request: &request, Response: &response}
	ctxURL := model.URL{Original: &origURL}
	custom := model.Custom{"abc": 1}

	inputMetadata := model.Metadata{Service: model.Service{Name: "foo"}}

	mergedMetadata := inputMetadata
	mergedMetadata.User = model.User{Name: name, Email: email, ID: userID}
	mergedMetadata.UserAgent.Original = ua
	mergedMetadata.Client.IP = net.ParseIP(userIP)

	// baseInput holds the minimal valid input. Test-specific input is added to this.
	baseInput := map[string]interface{}{
		"id": id, "type": trType, "name": name, "duration": duration, "trace_id": traceID,
		"span_count": map[string]interface{}{"dropped": 12.0, "started": 6.0},
	}

	for name, test := range map[string]struct {
		input map[string]interface{}
		cfg   Config
		e     *model.Transaction
	}{
		"no timestamp specified, request time used": {
			input: map[string]interface{}{},
			e: &model.Transaction{
				Metadata:  inputMetadata,
				ID:        id,
				Type:      trType,
				Name:      name,
				TraceID:   traceID,
				Duration:  duration,
				Timestamp: requestTime,
				SpanCount: model.SpanCount{Dropped: &dropped, Started: &started},
				Outcome:   "unknown",
			},
		},
		"event experimental=true, no experimental payload": {
			input: map[string]interface{}{
				"timestamp": timestampEpoch,
				"context":   map[string]interface{}{"foo": "bar"},
			},
			cfg: Config{Experimental: true},
			e: &model.Transaction{
				Metadata:  inputMetadata,
				ID:        id,
				Type:      trType,
				Name:      name,
				TraceID:   traceID,
				Duration:  duration,
				Timestamp: timestampParsed,
				SpanCount: model.SpanCount{Dropped: &dropped, Started: &started},
				Outcome:   "unknown",
			},
		},
		"event experimental=false": {
			input: map[string]interface{}{
				"timestamp": timestampEpoch,
				"context":   map[string]interface{}{"experimental": map[string]interface{}{"foo": "bar"}},
			},
			cfg: Config{Experimental: false},
			e: &model.Transaction{
				Metadata:  inputMetadata,
				ID:        id,
				Type:      trType,
				Name:      name,
				TraceID:   traceID,
				Duration:  duration,
				Timestamp: timestampParsed,
				SpanCount: model.SpanCount{Dropped: &dropped, Started: &started},
				Outcome:   "unknown",
			},
		},
		"event experimental=true": {
			input: map[string]interface{}{
				"timestamp": timestampEpoch,
				"context":   map[string]interface{}{"experimental": map[string]interface{}{"foo": "bar"}},
			},
			cfg: Config{Experimental: true},
			e: &model.Transaction{
				Metadata:     inputMetadata,
				ID:           id,
				Type:         trType,
				Name:         name,
				TraceID:      traceID,
				Duration:     duration,
				Timestamp:    timestampParsed,
				SpanCount:    model.SpanCount{Dropped: &dropped, Started: &started},
				Experimental: map[string]interface{}{"foo": "bar"},
				Outcome:      "unknown",
			},
		},
		"messaging event": {
			input: map[string]interface{}{
				"timestamp": timestampEpoch,
				"type":      "messaging",
				"context": map[string]interface{}{
					"message": map[string]interface{}{
						"queue":   map[string]interface{}{"name": "order"},
						"body":    "confirmed",
						"headers": map[string]interface{}{"internal": "false"},
						"age":     map[string]interface{}{"ms": json.Number("1577958057123")},
					},
				},
			},
			e: &model.Transaction{
				Metadata:  inputMetadata,
				ID:        id,
				Name:      name,
				Type:      "messaging",
				TraceID:   traceID,
				Duration:  duration,
				Timestamp: timestampParsed,
				SpanCount: model.SpanCount{Dropped: &dropped, Started: &started},
				Outcome:   "unknown",
				Message: &model.Message{
					QueueName: tests.StringPtr("order"),
					Body:      tests.StringPtr("confirmed"),
					Headers:   http.Header{"Internal": []string{"false"}},
					AgeMillis: tests.IntPtr(1577958057123),
				},
			},
		},
		"valid event": {
			input: map[string]interface{}{
				"timestamp": timestampEpoch,
				"result":    result,
				"outcome":   outcome,
				"sampled":   sampled,
				"parent_id": parentID,
				"marks":     marks,
				"context": map[string]interface{}{
					"a":      "b",
					"custom": map[string]interface{}{"abc": 1},
					"user":   map[string]interface{}{"username": name, "email": email, "ip": userIP, "id": userID},
					"tags":   map[string]interface{}{"foo": "bar"},
					"page":   map[string]interface{}{"url": url, "referer": referer},
					"request": map[string]interface{}{
						"method":  "POST",
						"url":     map[string]interface{}{"raw": "127.0.0.1"},
						"headers": map[string]interface{}{"user-agent": ua},
					},
					"response": map[string]interface{}{
						"finished": false,
						"headers":  map[string]interface{}{"Content-Type": "text/html"},
					},
				},
				"experience": map[string]interface{}{
					"cls":     1.0,
					"fid":     2.3,
					"ignored": 4,
				},
			},
			e: &model.Transaction{
				Metadata:  mergedMetadata,
				ID:        id,
				Type:      trType,
				Name:      name,
				Result:    result,
				Outcome:   outcome,
				ParentID:  parentID,
				TraceID:   traceID,
				Duration:  duration,
				Timestamp: timestampParsed,
				Marks: model.TransactionMarks{
					"navigationTiming": model.TransactionMark{
						"appBeforeBootstrap": 608.9300000000001,
						"navigationStart":    -21,
					},
				},
				Sampled:   &sampled,
				SpanCount: model.SpanCount{Dropped: &dropped, Started: &started},
				Labels:    &labels,
				Page:      &page,
				Custom:    &custom,
				HTTP:      &h,
				URL:       &ctxURL,
				UserExperience: &model.UserExperience{
					CumulativeLayoutShift: 1,
					FirstInputDelay:       2.3,
					TotalBlockingTime:     -1, // undefined
				},
			},
		},
		"with derived success outcome": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"response": map[string]interface{}{
						"status_code": json.Number("400"),
					},
				},
			},
			e: &model.Transaction{
				Metadata: inputMetadata,
				ID:       id,
				Type:     trType,
				Name:     name,
				TraceID:  traceID,
				Duration: duration,
				HTTP: &model.Http{Response: &model.Resp{
					MinimalResp: model.MinimalResp{StatusCode: &badRequestResp},
				}},
				Timestamp: requestTime,
				SpanCount: model.SpanCount{Dropped: &dropped, Started: &started},
				Outcome:   "success",
			},
		},
		"with derived failure outcome": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"response": map[string]interface{}{
						"status_code": json.Number("500"),
					},
				},
			},
			e: &model.Transaction{
				Metadata:  inputMetadata,
				ID:        id,
				Type:      trType,
				Name:      name,
				TraceID:   traceID,
				Duration:  duration,
				Timestamp: requestTime,
				HTTP: &model.Http{Response: &model.Resp{
					MinimalResp: model.MinimalResp{StatusCode: &internalErrorResp},
				}},
				SpanCount: model.SpanCount{Dropped: &dropped, Started: &started},
				Outcome:   "failure",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := make(map[string]interface{})
			for k, v := range baseInput {
				input[k] = v
			}
			for k, v := range test.input {
				input[k] = v
			}
			batch := &model.Batch{}
			err := DecodeTransaction(Input{
				Raw:         input,
				RequestTime: requestTime,
				Metadata:    inputMetadata,
				Config:      test.cfg,
			}, batch)
			require.NoError(t, err)
			assert.Equal(t, test.e, batch.Transactions[0])
		})
	}
}

func BenchmarkDecodeTransaction(b *testing.B) {
	var fullMetadata model.Metadata
	require.NoError(b, DecodeMetadata(fullInput, false, &fullMetadata))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := DecodeTransaction(Input{
			Metadata: fullMetadata,
			Raw:      fullTransactionInput,
		}, &model.Batch{}); err != nil {
			b.Fatal(err)
		}
	}
}
