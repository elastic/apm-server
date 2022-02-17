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
	Log         Log

	// Timestamp holds the event timestamp.
	//
	// See https://www.elastic.co/guide/en/ecs/current/ecs-base.html#field-timestamp
	Timestamp time.Time

	// Labels holds the string (keyword) labels to apply to the event, stored as
	// keywords. Supports slice values.
	//
	// See https://www.elastic.co/guide/en/ecs/current/ecs-base.html#field-labels
	Labels Labels

	// NumericLabels holds the numeric (scaled_float) labels to apply to the event.
	// Supports slice values.
	NumericLabels NumericLabels

	// Message holds the message for log events.
	//
	// See https://www.elastic.co/guide/en/ecs/current/ecs-base.html#field-message
	Message string

	Transaction   *Transaction
	Span          *Span
	Metricset     *Metricset
	Error         *Error
	ProfileSample *ProfileSample
}

// BeatEvent converts e to a beat.Event.
func (e *APMEvent) BeatEvent(ctx context.Context) beat.Event {
	event := beat.Event{
		Timestamp: e.Timestamp,
		Fields:    make(common.MapStr),
	}
	fields := (*mapStr)(&event.Fields)

	if e.Transaction != nil {
		fields.maybeSetMapStr("transaction", e.Transaction.fields())
	}
	if e.Span != nil {
		fields.maybeSetMapStr("span", e.Span.fields())
	}
	if e.Metricset != nil {
		e.Metricset.setFields((*mapStr)(&event.Fields))
	}
	if e.Error != nil {
		fields.maybeSetMapStr("error", e.Error.fields())
	}
	if e.ProfileSample != nil {
		fields.maybeSetMapStr("profile", e.ProfileSample.fields())
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
	event.Timestamp = e.Timestamp
	e.DataStream.setFields(fields)
	if e.ECSVersion != "" {
		fields.set("ecs", common.MapStr{"version": e.ECSVersion})
	}
	fields.maybeSetMapStr("service", e.Service.Fields())
	fields.maybeSetMapStr("agent", e.Agent.fields())
	fields.maybeSetMapStr("observer", e.Observer.Fields())
	fields.maybeSetMapStr("host", e.Host.fields())
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
	fields.maybeSetMapStr("labels", e.Labels.fields())
	fields.maybeSetMapStr("numeric_labels", e.NumericLabels.fields())
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
	fields.maybeSetMapStr("log", e.Log.fields())
	return event
}
