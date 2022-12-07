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
	"time"

	"go.elastic.co/fastjson"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/elastic-agent-libs/mapstr"
)

const (
	// timestampFormat formats timestamps according to Elasticsearch's
	// strict_date_optional_time date format, which includes a fractional
	// seconds component.
	timestampFormat = "2006-01-02T15:04:05.000Z07:00"
)

// APMEvent holds the details of an APM event.
//
// Exactly one of the event fields should be non-nil.
type APMEvent struct {
	// DataStream optionally holds data stream identifiers.
	DataStream DataStream

	Event       Event
	Agent       Agent
	Observer    Observer
	Container   Container
	Kubernetes  Kubernetes
	Service     Service
	Process     Process
	Device      Device
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

	Transaction *Transaction
	Span        *Span
	Metricset   *Metricset
	Error       *Error
}

// MarshalJSON marshals e as JSON.
func (e *APMEvent) MarshalJSON() ([]byte, error) {
	var w fastjson.Writer
	if err := e.MarshalFastJSON(&w); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

// MarshalFastJSON marshals e as JSON, writing the result to w.
func (e *APMEvent) MarshalFastJSON(w *fastjson.Writer) error {
	return encodeBeatEvent(e.BeatEvent(), w)
}

func encodeBeatEvent(in beat.Event, out *fastjson.Writer) error {
	out.RawByte('{')
	out.RawString(`"@timestamp":"`)
	out.Time(in.Timestamp, timestampFormat)
	out.RawByte('"')
	for k, v := range in.Fields {
		out.RawByte(',')
		out.String(k)
		out.RawByte(':')
		if err := encodeAny(v, out); err != nil {
			return err
		}
	}
	out.RawByte('}')
	return nil
}

func encodeAny(v interface{}, out *fastjson.Writer) error {
	switch v := v.(type) {
	case mapstr.M:
		return encodeMap(v, out)
	case map[string]interface{}:
		return encodeMap(v, out)
	default:
		return fastjson.Marshal(out, v)
	}
}

func encodeMap(v map[string]interface{}, out *fastjson.Writer) error {
	out.RawByte('{')
	first := true
	for k, v := range v {
		if first {
			first = false
		} else {
			out.RawByte(',')
		}
		out.String(k)
		out.RawByte(':')
		if err := encodeAny(v, out); err != nil {
			return err
		}
	}
	out.RawByte('}')
	return nil
}

// BeatEvent converts e to a beat.Event.
func (e *APMEvent) BeatEvent() beat.Event {
	event := beat.Event{
		Timestamp: e.Timestamp,
		Fields:    make(mapstr.M),
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

	// Set high resolution timestamp.
	//
	// TODO(axw) change @timestamp to use date_nanos, and remove this field.
	if !e.Timestamp.IsZero() {
		switch e.Processor {
		case TransactionProcessor, SpanProcessor, ErrorProcessor:
			event.Fields["timestamp"] = mapstr.M{"us": int(e.Timestamp.UnixNano() / 1000)}
		}
	}

	// Set top-level field sets.
	event.Timestamp = e.Timestamp
	e.DataStream.setFields(fields)
	fields.maybeSetMapStr("service", e.Service.Fields())
	fields.maybeSetMapStr("agent", e.Agent.fields())
	fields.maybeSetMapStr("observer", e.Observer.Fields())
	fields.maybeSetMapStr("host", e.Host.fields())
	fields.maybeSetMapStr("device", e.Device.fields())
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

// MarkGlobalLabels marks the current labels as "Global". This is only done
// after the agent defined labels have been decoded.
func (e *APMEvent) MarkGlobalLabels() {
	for key, v := range e.NumericLabels {
		v.Global = true
		e.NumericLabels[key] = v
	}
	for key, v := range e.Labels {
		v.Global = true
		e.Labels[key] = v
	}
}
