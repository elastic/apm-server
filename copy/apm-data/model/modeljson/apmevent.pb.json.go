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

package modeljson

import (
	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
	"go.elastic.co/fastjson"
)

func MarshalAPMEvent(e *modelpb.APMEvent, w *fastjson.Writer) error {
	var labels map[string]modeljson.Label
	if n := len(e.Labels); n > 0 {
		labels = make(map[string]modeljson.Label)
		for k, label := range e.Labels {
			if label != nil {
				labels[sanitizeLabelKey(k)] = modeljson.Label{
					Value:  label.Value,
					Values: label.Values,
				}
			}
		}
	}

	var numericLabels map[string]modeljson.NumericLabel
	if n := len(e.NumericLabels); n > 0 {
		numericLabels = make(map[string]modeljson.NumericLabel)
		for k, label := range e.NumericLabels {
			if label != nil {
				numericLabels[sanitizeLabelKey(k)] = modeljson.NumericLabel{
					Value:  label.Value,
					Values: label.Values,
				}
			}
		}
	}

	doc := modeljson.Document{
		Timestamp:     modeljson.Time(modelpb.ToTime(e.Timestamp)),
		Labels:        labels,
		NumericLabels: numericLabels,
		Message:       e.Message,
	}

	var dataStream modeljson.DataStream
	if e.DataStream != nil {
		doc.DataStream = &dataStream
		doc.DataStream.Type = e.DataStream.Type
		doc.DataStream.Dataset = e.DataStream.Dataset
		doc.DataStream.Namespace = e.DataStream.Namespace
	}

	var transaction modeljson.Transaction
	if e.Transaction != nil {
		TransactionModelJSON(e.Transaction, &transaction, e.Metricset != nil)
		doc.Transaction = &transaction
	}

	var span modeljson.Span
	if e.Span != nil {
		SpanModelJSON(e.Span, &span)
		doc.Span = &span
	}

	var metricset modeljson.Metricset
	if e.Metricset != nil {
		MetricsetModelJSON(e.Metricset, &metricset)
		doc.Metricset = &metricset
		doc.DocCount = e.Metricset.DocCount
	}

	var errorStruct modeljson.Error
	if e.Error != nil {
		ErrorModelJSON(e.Error, &errorStruct)
		doc.Error = &errorStruct
	}

	var event modeljson.Event
	if e.Event != nil {
		EventModelJSON(e.Event, &event)
		doc.Event = &event
	}

	// Set high resolution timestamp for transactions, spans, and errors.
	//
	// TODO(axw) change @timestamp to use date_nanos, and remove this field.
	var timestampStruct modeljson.Timestamp
	if e.Timestamp != 0 {
		switch e.Type() {
		case modelpb.TransactionEventType, modelpb.SpanEventType, modelpb.ErrorEventType:
			timestampStruct.US = int(e.Timestamp / 1000)
			doc.TimestampStruct = &timestampStruct
		}
	}

	var cloud modeljson.Cloud
	if e.Cloud != nil {
		CloudModelJSON(e.Cloud, &cloud)
		doc.Cloud = &cloud
	}

	var fass modeljson.FAAS
	if e.Faas != nil {
		FaasModelJSON(e.Faas, &fass)
		doc.FAAS = &fass
	}

	var device modeljson.Device
	if e.Device != nil {
		DeviceModelJSON(e.Device, &device)
		doc.Device = &device
	}

	var network modeljson.Network
	if e.Network != nil {
		NetworkModelJSON(e.Network, &network)
		doc.Network = &network
	}

	var observer modeljson.Observer
	if e.Observer != nil {
		ObserverModelJSON(e.Observer, &observer)
		doc.Observer = &observer
	}

	var container modeljson.Container
	if e.Container != nil {
		ContainerModelJSON(e.Container, &container)
		doc.Container = &container
	}

	var kubernetes modeljson.Kubernetes
	if e.Kubernetes != nil {
		KubernetesModelJSON(e.Kubernetes, &kubernetes)
		doc.Kubernetes = &kubernetes
	}

	var agent modeljson.Agent
	if e.Agent != nil {
		AgentModelJSON(e.Agent, &agent)
		doc.Agent = &agent
	}

	if e.Trace != nil {
		doc.Trace = &modeljson.Trace{
			ID: e.Trace.Id,
		}
	}

	var user modeljson.User
	if e.User != nil {
		UserModelJSON(e.User, &user)
		doc.User = &user
	}

	var source modeljson.Source
	if e.Source != nil {
		SourceModelJSON(e.Source, &source)
		doc.Source = &source
	}

	if len(e.ParentId) > 0 {
		doc.Parent = &modeljson.Parent{
			ID: e.ParentId,
		}
	}

	if len(e.ChildIds) > 0 {
		doc.Child = &modeljson.Child{
			ID: e.ChildIds,
		}
	}

	var client modeljson.Client
	if e.Client != nil {
		ClientModelJSON(e.Client, &client)
		doc.Client = &client
	}

	if e.UserAgent != nil {
		doc.UserAgent = &modeljson.UserAgent{
			Original: e.UserAgent.Original,
			Name:     e.UserAgent.Name,
		}
	}

	var service modeljson.Service
	if e.Service != nil {
		ServiceModelJSON(e.Service, &service)
		doc.Service = &service
	}

	var httpStruct modeljson.HTTP
	if e.Http != nil {
		HTTPModelJSON(e.Http, &httpStruct)
		doc.HTTP = &httpStruct
	}

	var host modeljson.Host
	if e.Host != nil {
		HostModelJSON(e.Host, &host)
		doc.Host = &host
	}

	var url modeljson.URL
	if e.Url != nil {
		URLModelJSON(e.Url, &url)
		doc.URL = &url
	}

	var logStruct modeljson.Log
	if e.Log != nil {
		LogModelJSON(e.Log, &logStruct)
		doc.Log = &logStruct
	}

	var process modeljson.Process
	if e.Process != nil {
		ProcessModelJSON(e.Process, &process)
		doc.Process = &process
	}

	var destination modeljson.Destination
	if e.Destination != nil {
		DestinationModelJSON(e.Destination, &destination)
		doc.Destination = &destination
	}

	if e.Session != nil {
		doc.Session = &modeljson.Session{
			ID:       e.Session.Id,
			Sequence: e.Session.Sequence,
		}
	}

	if e.Code != nil {
		doc.Code = &modeljson.Code{
			Stacktrace: e.Code.Stacktrace,
		}
	}

	var system modeljson.System
	if e.System != nil {
		SystemModelJSON(e.System, &system)
		doc.System = &system
	}

	return doc.MarshalFastJSON(w)
}
