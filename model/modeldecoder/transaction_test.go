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

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/transaction"
	"github.com/elastic/apm-server/tests"
)

func TestTransactionEventDecodeFailure(t *testing.T) {
	for name, test := range map[string]struct {
		input interface{}
		err   string
		e     *transaction.Event
	}{
		"no input":           {input: nil, err: "failed to validate transaction: error validating JSON: input missing", e: nil},
		"invalid type":       {input: "", err: "failed to validate transaction: error validating JSON: invalid input type", e: nil},
		"cannot fetch field": {input: map[string]interface{}{}, err: "failed to validate transaction: error validating JSON: (.|\n)*missing properties:(.|\n)*", e: nil},
	} {
		t.Run(name, func(t *testing.T) {
			transformable, err := DecodeTransaction(Input{Raw: test.input})
			if test.err != "" {
				assert.Regexp(t, test.err, err.Error())
			} else {
				assert.NoError(t, err)
			}
			if test.e != nil {
				event := transformable.(*transaction.Event)
				assert.Equal(t, test.e, event)
			} else {
				assert.Nil(t, transformable)
			}
		})
	}
}

func TestTransactionDecodeRUMV3Marks(t *testing.T) {
	// unknown fields are ignored
	input := map[string]interface{}{
		"k": map[string]interface{}{
			"foo": 0,
			"a": map[string]interface{}{
				"foo": 0,
				"dc":  1.2,
			},
			"nt": map[string]interface{}{
				"foo": 0,
				"dc":  1.2,
			},
		},
	}
	marks, err := decodeRUMV3Marks(input, Config{HasShortFieldNames: true})
	require.Nil(t, err)

	var f = 1.2
	assert.Equal(t, common.MapStr{
		"agent":            common.MapStr{"domComplete": &f},
		"navigationTiming": common.MapStr{"domComplete": &f},
	}, marks)
}

func TestTransactionEventDecode(t *testing.T) {
	id, trType, name, result := "123", "type", "foo()", "555"
	requestTime := time.Now()
	timestampParsed := time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)
	timestampEpoch := json.Number(fmt.Sprintf("%d", timestampParsed.UnixNano()/1000))

	traceID, parentID := "0147258369012345abcdef0123456789", "abcdef0123456789"
	dropped, started, duration := 12, 6, 1.67
	name, userID, email, userIP := "jane", "abc123", "j@d.com", "127.0.0.1"
	url, referer, origURL := "https://mypage.com", "http:mypage.com", "127.0.0.1"
	marks := map[string]interface{}{"navigationTiming": map[string]interface{}{
		"appBeforeBootstrap": 608.9300000000001,
		"navigationStart":    -21,
	}}
	sampled := true
	labels := model.Labels{"foo": "bar"}
	ua := "go-1.1"
	user := metadata.User{Name: name, Email: email, IP: net.ParseIP(userIP), Id: userID, UserAgent: ua}
	page := model.Page{Url: &url, Referer: &referer}
	request := model.Req{Method: "post", Socket: &model.Socket{}, Headers: http.Header{"User-Agent": []string{ua}}}
	response := model.Resp{Finished: new(bool), MinimalResp: model.MinimalResp{Headers: http.Header{"Content-Type": []string{"text/html"}}}}
	h := model.Http{Request: &request, Response: &response}
	ctxURL := model.Url{Original: &origURL}
	custom := model.Custom{"abc": 1}
	metadata := metadata.Metadata{Service: metadata.Service{Name: "foo"}}

	// baseInput holds the minimal valid input. Test-specific input is added to this.
	baseInput := map[string]interface{}{
		"id": id, "type": trType, "name": name, "duration": duration, "trace_id": traceID,
		"span_count": map[string]interface{}{"dropped": 12.0, "started": 6.0},
	}

	for name, test := range map[string]struct {
		input map[string]interface{}
		cfg   Config
		e     *transaction.Event
	}{
		"no timestamp specified, request time used": {
			input: map[string]interface{}{},
			e: &transaction.Event{
				Metadata:  metadata,
				Id:        id,
				Type:      trType,
				Name:      &name,
				TraceId:   traceID,
				Duration:  duration,
				Timestamp: requestTime,
				SpanCount: transaction.SpanCount{Dropped: &dropped, Started: &started},
			},
		},
		"event experimental=true, no experimental payload": {
			input: map[string]interface{}{
				"timestamp": timestampEpoch,
				"context":   map[string]interface{}{"foo": "bar"},
			},
			cfg: Config{Experimental: true},
			e: &transaction.Event{
				Metadata:  metadata,
				Id:        id,
				Type:      trType,
				Name:      &name,
				TraceId:   traceID,
				Duration:  duration,
				Timestamp: timestampParsed,
				SpanCount: transaction.SpanCount{Dropped: &dropped, Started: &started},
			},
		},
		"event experimental=false": {
			input: map[string]interface{}{
				"timestamp": timestampEpoch,
				"context":   map[string]interface{}{"experimental": map[string]interface{}{"foo": "bar"}},
			},
			cfg: Config{Experimental: false},
			e: &transaction.Event{
				Metadata:  metadata,
				Id:        id,
				Type:      trType,
				Name:      &name,
				TraceId:   traceID,
				Duration:  duration,
				Timestamp: timestampParsed,
				SpanCount: transaction.SpanCount{Dropped: &dropped, Started: &started},
			},
		},
		"event experimental=true": {
			input: map[string]interface{}{
				"timestamp": timestampEpoch,
				"context":   map[string]interface{}{"experimental": map[string]interface{}{"foo": "bar"}},
			},
			cfg: Config{Experimental: true},
			e: &transaction.Event{
				Metadata:     metadata,
				Id:           id,
				Type:         trType,
				Name:         &name,
				TraceId:      traceID,
				Duration:     duration,
				Timestamp:    timestampParsed,
				SpanCount:    transaction.SpanCount{Dropped: &dropped, Started: &started},
				Experimental: map[string]interface{}{"foo": "bar"},
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
			e: &transaction.Event{
				Metadata:  metadata,
				Id:        id,
				Name:      &name,
				Type:      "messaging",
				TraceId:   traceID,
				Duration:  duration,
				Timestamp: timestampParsed,
				SpanCount: transaction.SpanCount{Dropped: &dropped, Started: &started},
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
			},
			e: &transaction.Event{
				Metadata:  metadata,
				Id:        id,
				Type:      trType,
				Name:      &name,
				Result:    &result,
				ParentId:  &parentID,
				TraceId:   traceID,
				Duration:  duration,
				Timestamp: timestampParsed,
				Marks:     marks,
				Sampled:   &sampled,
				SpanCount: transaction.SpanCount{Dropped: &dropped, Started: &started},
				User:      &user,
				Labels:    &labels,
				Page:      &page,
				Custom:    &custom,
				Http:      &h,
				Url:       &ctxURL,
				Client:    &model.Client{IP: net.ParseIP(userIP)},
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

			transformable, err := DecodeTransaction(Input{
				Raw:         input,
				RequestTime: requestTime,
				Metadata:    metadata,
				Config:      test.cfg,
			})
			require.NoError(t, err)
			assert.Equal(t, test.e, transformable)
		})
	}
}
