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
	"net/http"
	"testing"
	"time"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/common"
)

func TestTransactionEventDecodeFailure(t *testing.T) {
	for name, test := range map[string]struct {
		input       interface{}
		err, inpErr error
		e           *Event
	}{
		"no input":           {input: nil, err: errMissingInput, e: nil},
		"input error":        {input: nil, inpErr: errors.New("a"), err: errors.New("a"), e: nil},
		"invalid type":       {input: "", err: errInvalidType, e: nil},
		"cannot fetch field": {input: map[string]interface{}{}, err: utility.ErrFetch, e: nil},
	} {
		t.Run(name, func(t *testing.T) {
			transformable, err := DecodeEvent(test.input, model.Config{}, test.inpErr)
			assert.Equal(t, test.err, err)
			if test.e != nil {
				event := transformable.(*Event)
				assert.Equal(t, test.e, event)
			} else {
				assert.Nil(t, transformable)
			}
		})
	}
}

func TestTransactionEventDecode(t *testing.T) {
	id, trType, name, result := "123", "type", "foo()", "555"
	timestampParsed := time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)
	timestampEpoch := json.Number(fmt.Sprintf("%d", timestampParsed.UnixNano()/1000))
	traceId, parentId := "0147258369012345abcdef0123456789", "abcdef0123456789"
	dropped, started, duration := 12, 6, 1.67
	name, userId, email, userIp := "jane", "abc123", "j@d.com", "127.0.0.1"
	url, referer, origUrl := "https://mypage.com", "http:mypage.com", "127.0.0.1"
	marks := map[string]interface{}{"k": "b"}
	sampled := true
	labels := model.Labels{"foo": "bar"}
	ua := "go-1.1"
	user := metadata.User{Name: &name, Email: &email, IP: &userIp, Id: &userId, UserAgent: &ua}
	page := model.Page{Url: &url, Referer: &referer}
	request := model.Req{Method: "post", Socket: &model.Socket{}, Headers: http.Header{"User-Agent": []string{ua}}}
	response := model.Resp{Finished: new(bool), Headers: http.Header{"Content-Type": []string{"text/html"}}}
	h := model.Http{Request: &request, Response: &response}
	ctxUrl := model.Url{Original: &origUrl}
	custom := model.Custom{"abc": 1}

	for name, test := range map[string]struct {
		input interface{}
		cfg   model.Config
		err   error
		e     *Event
	}{
		"event experimental=true, no experimental payload": {
			input: map[string]interface{}{
				"id": id, "type": trType, "name": name, "duration": duration, "trace_id": traceId,
				"timestamp": timestampEpoch, "context": map[string]interface{}{"foo": "bar"},
			},
			cfg: model.Config{Experimental: true},
			e: &Event{
				Id:        id,
				Type:      trType,
				Name:      &name,
				TraceId:   traceId,
				Duration:  duration,
				Timestamp: timestampParsed,
			},
		},
		"event experimental=false": {
			input: map[string]interface{}{
				"id": id, "type": trType, "name": name, "duration": duration, "trace_id": traceId, "timestamp": timestampEpoch,
				"context": map[string]interface{}{"experimental": map[string]interface{}{"foo": "bar"}},
			},
			cfg: model.Config{Experimental: false},
			e: &Event{
				Id:        id,
				Type:      trType,
				Name:      &name,
				TraceId:   traceId,
				Duration:  duration,
				Timestamp: timestampParsed,
			},
		},
		"event experimental=true": {
			input: map[string]interface{}{
				"id": id, "type": trType, "name": name, "duration": duration, "trace_id": traceId, "timestamp": timestampEpoch,
				"context": map[string]interface{}{"experimental": map[string]interface{}{"foo": "bar"}},
			},
			cfg: model.Config{Experimental: true},
			e: &Event{
				Id:           id,
				Type:         trType,
				Name:         &name,
				TraceId:      traceId,
				Duration:     duration,
				Timestamp:    timestampParsed,
				Experimental: map[string]interface{}{"foo": "bar"},
			},
		},
		"full event": {
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
						"headers": map[string]interface{}{"user-agent": ua}},
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
				Http:      &h,
				Url:       &ctxUrl,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			transformable, err := DecodeEvent(test.input, test.cfg, nil)
			assert.Equal(t, test.err, err)
			if test.e != nil && assert.NotNil(t, transformable) {
				event := transformable.(*Event)
				assert.Equal(t, test.e, event)
			}
		})
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
	timestamp := time.Date(2019, 1, 3, 15, 17, 4, 908.596*1e6, time.FixedZone("+0100", 3600))
	timestampUs := timestamp.UnixNano() / 1000
	id, name, ip, userAgent := "123", "jane", "63.23.123.4", "node-js-2.3"
	user := metadata.User{Id: &id, Name: &name, IP: &ip, UserAgent: &userAgent}
	url, referer := "https://localhost", "http://localhost"
	serviceName := "myservice"
	metadataLabels := common.MapStr{"a": true}
	service := metadata.Service{Name: &serviceName}
	system := func() *metadata.System {
		return &metadata.System{
			DetectedHostname: &hostname,
			Architecture:     &architecture,
			Platform:         &platform,
		}
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
		"labels":    common.MapStr{"a": true},
		"timestamp": common.MapStr{"us": timestampUs},
	}

	txValidWithSystem := common.MapStr{
		"host": common.MapStr{
			"architecture": architecture,
			"hostname":     hostname,
			"name":         hostname,
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
		"labels":    common.MapStr{"a": true},
		"timestamp": common.MapStr{"us": timestampUs},
		"transaction": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"id":       "",
			"type":     "",
			"sampled":  true,
		},
	}

	request := model.Req{Method: "post", Socket: &model.Socket{}, Headers: http.Header{}}
	response := model.Resp{Finished: new(bool), Headers: http.Header{"content-type": []string{"text/html"}}}
	txWithContext := Event{
		Timestamp: timestamp,
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
			"name":         "jane",
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
			"response": common.MapStr{"finished": false, "headers": common.MapStr{"content-type": []string{"text/html"}}}},
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
				nil, nil, nil, metadataLabels,
			),
			Event:  &txValid,
			Output: []common.MapStr{txValidEs},
			Msg:    "Payload with multiple Events",
		}, {
			Metadata: metadata.NewMetadata(
				&service,
				nil, nil, nil, metadataLabels,
			),
			Event:  &txValidWithSpan,
			Output: []common.MapStr{txValidEs, spanEs},
			Msg:    "Payload with multiple Events",
		},

		{
			Metadata: metadata.NewMetadata(
				&service, system(),
				nil, nil, metadataLabels,
			),
			Event:  &txValid,
			Output: []common.MapStr{txValidWithSystem},
			Msg:    "Payload with System and Event",
		},
		{
			Metadata: metadata.NewMetadata(
				&service, func() *metadata.System { s := system(); s.ConfiguredHostname = &name; return s }(),
				nil, nil, metadataLabels,
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
	reqTimestampParsed := time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)
	e := Event{}
	beatEvent := e.Transform(&transform.Context{RequestTime: reqTimestampParsed})
	require.Len(t, beatEvent, 1)
	assert.Equal(t, reqTimestampParsed, beatEvent[0].Timestamp)
}
