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

	Agent      Agent
	Container  Container
	Kubernetes Kubernetes
	Service    Service
	Process    Process
	Host       Host
	User       User
	UserAgent  UserAgent
	Client     Client
	Cloud      Cloud
	Network    Network

	// Labels holds labels to apply to the event.
	//
	// TODO(axw) remove Transaction.Labels, Span.Labels, etc.,
	// and merge into these labels at decoding time. There can
	// be only one.
	Labels common.MapStr

	Transaction   *Transaction
	Span          *Span
	Metricset     *Metricset
	Error         *Error
	ProfileSample *ProfileSample
}

func (e *APMEvent) appendBeatEvent(ctx context.Context, out []beat.Event) []beat.Event {
	var event beat.Event
	var eventLabels common.MapStr
	switch {
	case e.Transaction != nil:
		event = e.Transaction.toBeatEvent()
		eventLabels = e.Transaction.Labels
	case e.Span != nil:
		event = e.Span.toBeatEvent(ctx)
		eventLabels = e.Span.Labels
	case e.Metricset != nil:
		event = e.Metricset.toBeatEvent()
		eventLabels = e.Metricset.Labels
	case e.Error != nil:
		event = e.Error.toBeatEvent(ctx)
		eventLabels = e.Error.Labels
	case e.ProfileSample != nil:
		event = e.ProfileSample.toBeatEvent()
		eventLabels = e.ProfileSample.Labels
	default:
		return out
	}

	// Set fields common to all events.
	fields := (*mapStr)(&event.Fields)
	e.DataStream.setFields(fields)
	fields.maybeSetMapStr("service", e.Service.Fields())
	fields.maybeSetMapStr("agent", e.Agent.fields())
	fields.maybeSetMapStr("host", e.Host.fields())
	fields.maybeSetMapStr("process", e.Process.fields())
	fields.maybeSetMapStr("user", e.User.fields())
	if client := e.Client.fields(); fields.maybeSetMapStr("client", client) {
		// We copy client to source for transactions and errors.
		if e.Transaction != nil || e.Error != nil {
			// TODO(axw) once we are using Fleet for ingest pipeline
			// management, move this to an ingest pipeline.
			fields.set("source", client)
		}
	}
	fields.maybeSetMapStr("user_agent", e.UserAgent.fields())
	fields.maybeSetMapStr("container", e.Container.fields())
	fields.maybeSetMapStr("kubernetes", e.Kubernetes.fields())
	fields.maybeSetMapStr("cloud", e.Cloud.fields())
	fields.maybeSetMapStr("network", e.Network.fields())
	maybeSetLabels(fields, e.Labels, eventLabels)
	return append(out, event)
}
