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

package transaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/elastic/apm-server/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/common"
)

func TestTransactionEventDecodeFailure(t *testing.T) {
	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		e           *Event
	}{
		{input: nil, err: errors.New("Input missing for decoding Event"), e: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), e: nil},
		{input: "", err: errors.New("Invalid type for transaction event"), e: nil},
		{input: map[string]interface{}{}, err: errors.New("Error fetching field"), e: nil},
	} {
		transformable, err := DecodeEvent(test.input, test.inpErr)
		assert.Equal(t, test.err, err)
		if test.e != nil {
			event := transformable.(*Event)
			assert.Equal(t, test.e, event)
		} else {
			assert.Nil(t, transformable)
		}
	}
}

func TestTransactionEventDecode(t *testing.T) {
	id, trType, name, result := "123", "type", "foo()", "555"
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)
	timestampEpoch := json.Number(fmt.Sprintf("%d", timestampParsed.UnixNano()/1000))
	traceId, parentId := "0147258369012345abcdef0123456789", "abcdef0123456789"
	dropped, started, duration := 12, 6, 1.67
	name, userId, email, userIp := "jane", "abc123", "j@d.com", "127.0.0.1"
	url, referer, origUrl := "https://mypage.com", "http:mypage.com", "127.0.0.1"
	marks := map[string]interface{}{"k": "b"}
	sampled := true
	labels := model.Labels{"foo": "bar"}
	user := metadata.User{Name: &name, Email: &email, IP: &userIp, Id: &userId}
	page := model.Page{Url: &url, Referer: &referer}
	request := model.Req{Method: "post", Socket: &model.Socket{}, Headers: &model.Headers{"user-agent": "go-1.1"}}
	response := model.Resp{Finished: new(bool), Headers: &model.Headers{"Content-Type": "text/html"}}
	http := model.Http{Request: &request, Response: &response}
	ctxUrl := model.Url{Original: &origUrl}
	custom := model.Custom{"abc": 1}
	context := model.Context{User: &user, Labels: &labels, Page: &page, Http: &http, Url: &ctxUrl, Custom: &custom}

	for _, test := range []struct {
		input interface{}
		err   error
		e     *Event
	}{

		// full event, ignoring spans
		{
			input: map[string]interface{}{
				"id": id, "type": trType, "name": name, "result": result,
				"duration": duration, "timestamp": timestampEpoch,
				"context": map[string]interface{}{
					"a":      "b",
					"custom": map[string]interface{}{"abc": 1},
					"user":   map[string]interface{}{"username": name, "email": email, "ip": userIp, "id": userId},
					"tags":   map[string]interface{}{"foo": "bar"},
					"page":   map[string]interface{}{"url": url, "referer": referer},
					"request": map[string]interface{}{
						"method":  "POST",
						"url":     map[string]interface{}{"raw": "127.0.0.1"},
						"headers": map[string]interface{}{"user-agent": "go-1.1"}},
					"response": map[string]interface{}{
						"finished": false,
						"headers":  map[string]interface{}{"Content-Type": "text/html"}},
				},
				"marks": marks, "sampled": sampled,
				"parent_id": parentId, "trace_id": traceId,
				"spans": []interface{}{
					map[string]interface{}{
						"name": "span", "type": "db", "start": 1.2, "duration": 2.3,
					}},
				"span_count": map[string]interface{}{"dropped": 12.0, "started": 6.0}},
			e: &Event{
				Id:        id,
				Type:      trType,
				Name:      &name,
				Result:    &result,
				ParentId:  &parentId,
				TraceId:   traceId,
				Duration:  duration,
				Timestamp: timestampParsed,
				Marks:     marks,
				Sampled:   &sampled,
				SpanCount: SpanCount{Dropped: &dropped, Started: &started},
				User:      &user,
				Labels:    &labels,
				Page:      &page,
				Custom:    &custom,
				Http:      &http,
				Url:       &ctxUrl,
				Context:   &context,
			},
		},
	} {
		transformable, err := DecodeEvent(test.input, nil)
		assert.Equal(t, test.err, err)
		if test.e != nil && assert.NotNil(t, transformable) {
			event := transformable.(*Event)
			assert.Equal(t, test.e, event)
		}
	}
}

func TestEventTransform(t *testing.T) {

	id := "123"
	result := "tx result"
	sampled := false
	dropped, startedSpans := 5, 14
	name := "mytransaction"

	tests := []struct {
		Event  Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event: Event{},
			Output: common.MapStr{
				"id":       "",
				"type":     "",
				"duration": common.MapStr{"us": 0},
				"sampled":  true,
			},
			Msg: "Empty Event",
		},
		{
			Event: Event{
				Id:       id,
				Type:     "tx",
				Duration: 65.98,
			},
			Output: common.MapStr{
				"id":       id,
				"type":     "tx",
				"duration": common.MapStr{"us": 65980},
				"sampled":  true,
			},
			Msg: "SpanCount empty",
		},
		{
			Event: Event{
				Id:        id,
				Type:      "tx",
				Duration:  65.98,
				SpanCount: SpanCount{Started: &startedSpans},
			},
			Output: common.MapStr{
				"id":         id,
				"type":       "tx",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"started": 14},
				"sampled":    true,
			},
			Msg: "SpanCount only contains `started`",
		},
		{
			Event: Event{
				Id:        id,
				Type:      "tx",
				Duration:  65.98,
				SpanCount: SpanCount{Dropped: &dropped},
			},
			Output: common.MapStr{
				"id":         id,
				"type":       "tx",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"dropped": 5},
				"sampled":    true,
			},
			Msg: "SpanCount only contains `dropped`",
		},
		{
			Event: Event{
				Id:        id,
				Name:      &name,
				Type:      "tx",
				Result:    &result,
				Timestamp: time.Now(),
				Duration:  65.98,
				Context:   &model.Context{},
				Sampled:   &sampled,
				SpanCount: SpanCount{Started: &startedSpans, Dropped: &dropped},
			},
			Output: common.MapStr{
				"id":         id,
				"name":       "mytransaction",
				"type":       "tx",
				"result":     "tx result",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"started": 14, "dropped": 5},
				"sampled":    false,
			},
			Msg: "Full Event",
		},
	}

	tctx := &transform.Context{}

	for idx, test := range tests {
		output := test.Event.Transform(tctx)
		assert.Equal(t, test.Output, output[0].Fields["transaction"], fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestEventsTransformWithMetadata(t *testing.T) {
	hostname := "a.b.c"
	architecture := "darwin"
	platform := "x64"
	timestamp, _ := time.Parse(time.RFC3339, "2019-01-03T15:17:04.908596+01:00")
	timestampUs := timestamp.UnixNano() / 1000
	id, name, ip, userAgent := "123", "jane", "63.23.123.4", "node-js-2.3"
	user := metadata.User{Id: &id, Name: &name, IP: &ip, UserAgent: &userAgent}
	url, referer := "https://localhost", "http://localhost"
	serviceName := "myservice"

	service := metadata.Service{Name: &serviceName}
	system := &metadata.System{
		Hostname:     &hostname,
		Architecture: &architecture,
		Platform:     &platform,
	}

	txValid := Event{Timestamp: timestamp}
	txValidEs := common.MapStr{
		"processor": common.MapStr{
			"event": "transaction",
			"name":  "transaction",
		},
		"service": common.MapStr{
			"name": "myservice",
		},
		"transaction": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"id":       "",
			"type":     "",
			"sampled":  true,
		},
		"timestamp": common.MapStr{"us": timestampUs},
	}

	txValidWithSystem := common.MapStr{
		"host": common.MapStr{
			"architecture": architecture,
			"hostname":     hostname,
			"os": common.MapStr{
				"platform": platform,
			},
		},
		"processor": common.MapStr{
			"event": "transaction",
			"name":  "transaction",
		},
		"service": common.MapStr{
			"name": "myservice",
		},
		"timestamp": common.MapStr{"us": timestampUs},
		"transaction": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"id":       "",
			"type":     "",
			"sampled":  true,
		},
	}

	request := model.Req{Method: "post", Socket: &model.Socket{}, Headers: &model.Headers{}}
	response := model.Resp{Finished: new(bool), Headers: &model.Headers{"content-type": "text/html"}}
	txWithContext := Event{
		Timestamp: timestamp,
		Context:   &model.Context{User: &user},
		User:      &user,
		Labels:    &model.Labels{"a": "b"},
		Page:      &model.Page{Url: &url, Referer: &referer},
		Http:      &model.Http{Request: &request, Response: &response},
		Url:       &model.Url{Original: &url},
		Custom:    &model.Custom{"foo": "bar"},
	}
	txWithContextEs := common.MapStr{
		"user":       common.MapStr{"id": "123", "name": "jane"},
		"client":     common.MapStr{"ip": "63.23.123.4"},
		"user_agent": common.MapStr{"original": userAgent},
		"host": common.MapStr{
			"architecture": "darwin",
			"hostname":     "a.b.c",
			"os": common.MapStr{
				"platform": "x64",
			},
		},
		"processor": common.MapStr{
			"event": "transaction",
			"name":  "transaction",
		},
		"service": common.MapStr{
			"name": "myservice",
		},
		"timestamp": common.MapStr{"us": timestampUs},
		"transaction": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"id":       "",
			"type":     "",
			"sampled":  true,
			"page":     common.MapStr{"url": url, "referer": referer},
			"custom": common.MapStr{
				"foo": "bar",
			},
		},
		"labels": common.MapStr{"a": "b"},
		"url":    common.MapStr{"original": url},
		"http": common.MapStr{
			"request":  common.MapStr{"method": "post"},
			"response": common.MapStr{"finished": false, "headers": common.MapStr{"content-type": "text/html"}}},
	}

	txValidWithSpan := Event{Timestamp: timestamp}
	spanEs := common.MapStr{
		"processor": common.MapStr{
			"event": "span",
			"name":  "transaction",
		},
		"service": common.MapStr{
			"name": "myservice",
		},
		"span": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"name":     "",
			"type":     "",
		},
		"timestamp": common.MapStr{"us": timestampUs},
	}

	tests := []struct {
		Metadata *metadata.Metadata
		Event    transform.Transformable
		Output   []common.MapStr
		Msg      string
	}{
		{
			Metadata: metadata.NewMetadata(&service,
				nil, nil, nil,
			),
			Event:  &txValid,
			Output: []common.MapStr{txValidEs},
			Msg:    "Payload with multiple Events",
		}, {
			Metadata: metadata.NewMetadata(
				&service,
				nil, nil, nil,
			),
			Event:  &txValidWithSpan,
			Output: []common.MapStr{txValidEs, spanEs},
			Msg:    "Payload with multiple Events",
		},

		{
			Metadata: metadata.NewMetadata(
				&service, system,
				nil, nil,
			),
			Event:  &txValid,
			Output: []common.MapStr{txValidWithSystem},
			Msg:    "Payload with System and Event",
		},
		{
			Metadata: metadata.NewMetadata(
				&service, system,
				nil, nil,
			),
			Event:  &txWithContext,
			Output: []common.MapStr{txWithContextEs},
			Msg:    "Payload with Service, System and Event with context",
		},
	}

	for idx, test := range tests {
		tctx := &transform.Context{
			Metadata:    *test.Metadata,
			RequestTime: timestamp,
		}
		outputEvents := test.Event.Transform(tctx)

		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v (j: %v); %s", idx, j, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Failed at idx %v (j: %v); %s", idx, j, test.Msg))
		}
	}
}

func TestEventTransformUseReqTime(t *testing.T) {
	reqTimestamp := "2017-05-30T18:53:27.154Z"
	reqTimestampParsed, err := time.Parse(time.RFC3339, reqTimestamp)
	require.NoError(t, err)

	e := Event{}
	beatEvent := e.Transform(&transform.Context{RequestTime: reqTimestampParsed})
	require.Len(t, beatEvent, 1)
	assert.Equal(t, reqTimestampParsed, beatEvent[0].Timestamp)
}
