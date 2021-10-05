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

package model

import (
	"context"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

// APMEvent holds the details of an APM event.
//
// Exactly one of the event fields should be non-nil.
type APMEvent struct {
	// DataStream optionally holds data stream identifiers.
	//
	// This will have the zero value when APM Server is run
	// in standalone mode.
	DataStream DataStream

	ECSVersion  string
	Event       Event
	Agent       Agent
	Observer    Observer
	Container   Container
	Kubernetes  Kubernetes
	Service     Service
	Process     Process
	Host        Host
	User        User
	UserAgent   UserAgent
	Client      Client
	Source      Source
	Destination Destination
	Cloud       Cloud
	Network     Network
	Session     Session
	URL         URL
	Processor   Processor
	Trace       Trace
	Parent      Parent
	Child       Child
	HTTP        HTTP
	FAAS        FAAS

	// Timestamp holds the event timestamp.
	//
	// See https://www.elastic.co/guide/en/ecs/current/ecs-base.html
	Timestamp time.Time

	// Labels holds labels to apply to the event.
	//
	// See https://www.elastic.co/guide/en/ecs/current/ecs-base.html
	Labels common.MapStr

	// Message holds the message for log events.
	//
	// See https://www.elastic.co/guide/en/ecs/current/ecs-base.html
	Message string

	Transaction   *Transaction
	Span          *Span
	Metricset     *Metricset
	Error         *Error
	ProfileSample *ProfileSample
	Firehose      *Firehose
}

// BeatEvent converts e to a beat.Event.
func (e *APMEvent) BeatEvent(ctx context.Context) beat.Event {
	event := beat.Event{
		Timestamp: e.Timestamp,
		Fields:    make(common.MapStr),
	}
	if e.Transaction != nil {
		e.Transaction.setFields((*mapStr)(&event.Fields), e)
	}
	if e.Span != nil {
		e.Span.setFields((*mapStr)(&event.Fields), e)
	}
	if e.Metricset != nil {
		e.Metricset.setFields((*mapStr)(&event.Fields))
	}
	if e.Error != nil {
		e.Error.setFields((*mapStr)(&event.Fields))
	}
	if e.ProfileSample != nil {
		e.ProfileSample.setFields((*mapStr)(&event.Fields))
	}
	if e.Firehose != nil {
		e.Firehose.setFields((*mapStr)(&event.Fields))
	}

	// Set high resolution timestamp.
	//
	// TODO(axw) change @timestamp to use date_nanos, and remove this field.
	if !e.Timestamp.IsZero() {
		switch e.Processor {
		case TransactionProcessor, SpanProcessor, ErrorProcessor:
			event.Fields["timestamp"] = common.MapStr{"us": int(e.Timestamp.UnixNano() / 1000)}
		}
	}

	// Set top-level field sets.
	fields := (*mapStr)(&event.Fields)
	event.Timestamp = e.Timestamp
	e.DataStream.setFields(fields)
	if e.ECSVersion != "" {
		fields.set("ecs", common.MapStr{"version": e.ECSVersion})
	}
	fields.maybeSetMapStr("service", e.Service.Fields())
	fields.maybeSetMapStr("agent", e.Agent.fields())
	fields.maybeSetMapStr("observer", e.Observer.Fields())
	if e.Firehose != nil {
		fields.maybeSetMapStr("host", e.Host.fields())
	}
	fields.maybeSetMapStr("process", e.Process.fields())
	fields.maybeSetMapStr("user", e.User.fields())
	fields.maybeSetMapStr("client", e.Client.fields())
	fields.maybeSetMapStr("source", e.Source.fields())
	fields.maybeSetMapStr("destination", e.Destination.fields())
	fields.maybeSetMapStr("user_agent", e.UserAgent.fields())
	fields.maybeSetMapStr("container", e.Container.fields())
	fields.maybeSetMapStr("kubernetes", e.Kubernetes.fields())
	fields.maybeSetMapStr("cloud", e.Cloud.fields())
	fields.maybeSetMapStr("network", e.Network.fields())
	fields.maybeSetMapStr("labels", sanitizeLabels(e.Labels))
	fields.maybeSetMapStr("event", e.Event.fields())
	fields.maybeSetMapStr("url", e.URL.fields())
	fields.maybeSetMapStr("session", e.Session.fields())
	fields.maybeSetMapStr("parent", e.Parent.fields())
	fields.maybeSetMapStr("child", e.Child.fields())
	fields.maybeSetMapStr("processor", e.Processor.fields())
	fields.maybeSetMapStr("trace", e.Trace.fields())
	fields.maybeSetString("message", e.Message)
	fields.maybeSetMapStr("http", e.HTTP.fields())
	fields.maybeSetMapStr("faas", e.FAAS.fields())
	if e.Processor == SpanProcessor {
		// Deprecated: copy url.original and http.* to span.http.* for backwards compatibility.
		//
		// TODO(axw) remove this in 8.0: https://github.com/elastic/apm-server/issues/5995
		var spanHTTPFields mapStr
		spanHTTPFields.maybeSetString("version", e.HTTP.Version)
		if e.HTTP.Request != nil {
			spanHTTPFields.maybeSetString("method", e.HTTP.Request.Method)
		}
		if e.HTTP.Response != nil {
			spanHTTPFields.maybeSetMapStr("response", e.HTTP.Response.fields())
		}
		if len(spanHTTPFields) != 0 || e.URL.Original != "" {
			spanFieldsMap, ok := event.Fields["span"].(common.MapStr)
			if !ok {
				spanFieldsMap = make(common.MapStr)
				event.Fields["span"] = spanFieldsMap
			}
			spanFields := mapStr(spanFieldsMap)
			spanFields.maybeSetMapStr("http", common.MapStr(spanHTTPFields))
			spanFields.maybeSetString("http.url.original", e.URL.Original)
		}
	}
	return event
}
