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

package codec

import (
	"testing"
	"time"

	"github.com/elastic/apm-data/model/modelpb"
)

func fullEvent(t *testing.B) *modelpb.APMEvent {
	return &modelpb.APMEvent{
		Timestamp: uint64(time.Second.Nanoseconds() + 1),
		Span: &modelpb.Span{
			Message: &modelpb.Message{
				Body: "body",
				Headers: []*modelpb.HTTPHeader{
					{
						Key:   "foo",
						Value: []string{"bar"},
					},
				},
				AgeMillis:  uint64Ptr(2),
				QueueName:  "queuename",
				RoutingKey: "routingkey",
			},
			Composite: &modelpb.Composite{
				CompressionStrategy: modelpb.CompressionStrategy_COMPRESSION_STRATEGY_EXACT_MATCH,
				Count:               1,
				Sum:                 2,
			},
			DestinationService: &modelpb.DestinationService{
				Type:     "destination_type",
				Name:     "destination_name",
				Resource: "destination_resource",
				ResponseTime: &modelpb.AggregatedDuration{
					Count: 3,
					Sum:   uint64(4 * time.Second),
				},
			},
			Db: &modelpb.DB{
				RowsAffected: uintPtr(5),
				Instance:     "db_instace",
				Statement:    "db_statement",
				Type:         "db_type",
				UserName:     "db_username",
				Link:         "db_link",
			},
			Sync:    boolPtr(true),
			Kind:    "kind",
			Action:  "action",
			Subtype: "subtype",
			Id:      "id",
			Type:    "type",
			Name:    "name",
			Stacktrace: []*modelpb.StacktraceFrame{
				{
					Vars:           randomKv(t),
					Lineno:         uintPtr(1),
					Colno:          uintPtr(2),
					Filename:       "frame_filename",
					Classname:      "frame_classname",
					ContextLine:    "frame_contextline",
					Module:         "frame_module",
					Function:       "frame_function",
					AbsPath:        "frame_abspath",
					SourcemapError: "frame_sourcemaperror",
					Original: &modelpb.Original{
						AbsPath:      "orig_abspath",
						Filename:     "orig_filename",
						Classname:    "orig_classname",
						Lineno:       uintPtr(3),
						Colno:        uintPtr(4),
						Function:     "orig_function",
						LibraryFrame: true,
					},
					PreContext:          []string{"pre"},
					PostContext:         []string{"post"},
					LibraryFrame:        true,
					SourcemapUpdated:    true,
					ExcludeFromGrouping: true,
				},
			},
			Links: []*modelpb.SpanLink{
				{
					TraceId: "trace_id",
					SpanId:  "id1",
				},
			},
			SelfTime: &modelpb.AggregatedDuration{
				Count: 6,
				Sum:   uint64(7 * time.Second),
			},
			RepresentativeCount: 8,
		},
		NumericLabels: map[string]*modelpb.NumericLabelValue{
			"foo": {
				Values: []float64{1, 2, 3},
				Value:  1,
				Global: true,
			},
		},
		Labels: map[string]*modelpb.LabelValue{
			"bar": {
				Value:  "a",
				Values: []string{"a", "b", "c"},
				Global: true,
			},
		},
		Message: "message",
		Transaction: &modelpb.Transaction{
			SpanCount: &modelpb.SpanCount{
				Started: uintPtr(1),
				Dropped: uintPtr(2),
			},
			UserExperience: &modelpb.UserExperience{
				CumulativeLayoutShift: 1,
				FirstInputDelay:       2,
				TotalBlockingTime:     3,
				LongTask: &modelpb.LongtaskMetrics{
					Count: 4,
					Sum:   5,
					Max:   6,
				},
			},
			// TODO investigat valid values
			Custom: nil,
			Marks: map[string]*modelpb.TransactionMark{
				"foo": {
					Measurements: map[string]float64{
						"bar": 3,
					},
				},
			},
			Message: &modelpb.Message{
				Body: "body",
				Headers: []*modelpb.HTTPHeader{
					{
						Key:   "foo",
						Value: []string{"bar"},
					},
				},
				AgeMillis:  uint64Ptr(2),
				QueueName:  "queuename",
				RoutingKey: "routingkey",
			},
			Type:   "type",
			Name:   "name",
			Result: "result",
			Id:     "id",
			DurationHistogram: &modelpb.Histogram{
				Values: []float64{4},
				Counts: []uint64{5},
			},
			DroppedSpansStats: []*modelpb.DroppedSpanStats{
				{
					DestinationServiceResource: "destinationserviceresource",
					ServiceTargetType:          "servicetargetype",
					ServiceTargetName:          "servicetargetname",
					Outcome:                    "outcome",
					Duration: &modelpb.AggregatedDuration{
						Count: 4,
						Sum:   uint64(5 * time.Second),
					},
				},
			},
			DurationSummary: &modelpb.SummaryMetric{
				Count: 6,
				Sum:   7,
			},
			RepresentativeCount: 8,
			Sampled:             true,
			Root:                true,
		},
		Metricset: &modelpb.Metricset{
			Name:     "name",
			Interval: "interval",
			Samples: []*modelpb.MetricsetSample{
				{
					Type: modelpb.MetricType_METRIC_TYPE_COUNTER,
					Name: "name",
					Unit: "unit",
					Histogram: &modelpb.Histogram{
						Values: []float64{1},
						Counts: []uint64{2},
					},
					Summary: &modelpb.SummaryMetric{
						Count: 3,
						Sum:   4,
					},
					Value: 5,
				},
			},
			DocCount: 1,
		},
		Error: &modelpb.Error{
			Exception: &modelpb.Exception{
				Message:    "ex_message",
				Module:     "ex_module",
				Code:       "ex_code",
				Attributes: randomKv(t),
				Type:       "ex_type",
				Handled:    boolPtr(true),
				Cause: []*modelpb.Exception{
					{
						Message: "ex1_message",
						Module:  "ex1_module",
						Code:    "ex1_code",
						Type:    "ex_type",
					},
				},
			},
			Log: &modelpb.ErrorLog{
				Message:      "log_message",
				Level:        "log_level",
				ParamMessage: "log_parammessage",
				LoggerName:   "log_loggername",
			},
			Id:          "id",
			GroupingKey: "groupingkey",
			Culprit:     "culprit",
			StackTrace:  "stacktrace",
			Message:     "message",
			Type:        "type",
		},
		Cloud: &modelpb.Cloud{
			Origin: &modelpb.CloudOrigin{
				AccountId:   "origin_accountid",
				Provider:    "origin_provider",
				Region:      "origin_region",
				ServiceName: "origin_servicename",
			},
			AccountId:        "accountid",
			AccountName:      "accountname",
			AvailabilityZone: "availabilityzone",
			InstanceId:       "instanceid",
			InstanceName:     "instancename",
			MachineType:      "machinetype",
			ProjectId:        "projectid",
			ProjectName:      "projectname",
			Provider:         "provider",
			Region:           "region",
			ServiceName:      "servicename",
		},
		Service: &modelpb.Service{
			Origin: &modelpb.ServiceOrigin{
				Id:      "origin_id",
				Name:    "origin_name",
				Version: "origin_version",
			},
			Target: &modelpb.ServiceTarget{
				Name: "target_name",
				Type: "target_type",
			},
			Language: &modelpb.Language{
				Name:    "language_name",
				Version: "language_version",
			},
			Runtime: &modelpb.Runtime{
				Name:    "runtime_name",
				Version: "runtime_version",
			},
			Framework: &modelpb.Framework{
				Name:    "framework_name",
				Version: "framework_version",
			},
			Name:        "name",
			Version:     "version",
			Environment: "environment",
			Node: &modelpb.ServiceNode{
				Name: "node_name",
			},
		},
		Faas: &modelpb.Faas{
			Id:               "id",
			ColdStart:        boolPtr(true),
			Execution:        "execution",
			TriggerType:      "triggertype",
			TriggerRequestId: "triggerrequestid",
			Name:             "name",
			Version:          "version",
		},
		Network: &modelpb.Network{
			Connection: &modelpb.NetworkConnection{
				Type:    "type",
				Subtype: "subtype",
			},
			Carrier: &modelpb.NetworkCarrier{
				Name: "name",
				Mcc:  "mcc",
				Mnc:  "mnc",
				Icc:  "icc",
			},
		},
		Container: &modelpb.Container{
			Id:        "id",
			Name:      "name",
			Runtime:   "runtime",
			ImageName: "imagename",
			ImageTag:  "imagetag",
		},
		User: &modelpb.User{
			Domain: "domain",
			Id:     "id",
			Email:  "email",
			Name:   "name",
		},
		Device: &modelpb.Device{
			Id: "id",
			Model: &modelpb.DeviceModel{
				Name:       "name",
				Identifier: "identifier",
			},
			Manufacturer: "manufacturer",
		},
		Kubernetes: &modelpb.Kubernetes{
			Namespace: "namespace",
			NodeName:  "nodename",
			PodName:   "podname",
			PodUid:    "poduid",
		},
		Observer: &modelpb.Observer{
			Hostname: "hostname",
			Name:     "name",
			Type:     "type",
			Version:  "version",
		},
		DataStream: &modelpb.DataStream{
			Type:      "type",
			Dataset:   "dataset",
			Namespace: "namespace",
		},
		Agent: &modelpb.Agent{
			Name:             "name",
			Version:          "version",
			EphemeralId:      "ephemeralid",
			ActivationMethod: "activationmethod",
		},
		Http: &modelpb.HTTP{
			Request: &modelpb.HTTPRequest{
				Headers:  randomHTTPHeaders(t),
				Env:      randomKv(t),
				Cookies:  randomKv(t),
				Id:       "id",
				Method:   "method",
				Referrer: "referrer",
			},
			Response: &modelpb.HTTPResponse{
				Headers:         randomHTTPHeaders(t),
				Finished:        boolPtr(true),
				HeadersSent:     boolPtr(true),
				TransferSize:    uint64Ptr(1),
				EncodedBodySize: uint64Ptr(2),
				DecodedBodySize: uint64Ptr(3),
				StatusCode:      200,
			},
			Version: "version",
		},
		UserAgent: &modelpb.UserAgent{
			Original: "original",
			Name:     "name",
		},
		ParentId: "id",
		Trace: &modelpb.Trace{
			Id: "id",
		},
		Host: &modelpb.Host{
			Os: &modelpb.OS{
				Name:     "name",
				Version:  "version",
				Platform: "platform",
				Full:     "full",
				Type:     "type",
			},
			Hostname:     "hostname",
			Name:         "name",
			Id:           "id",
			Architecture: "architecture",
			Type:         "type",
			Ip: []*modelpb.IP{
				modelpb.MustParseIP("127.0.0.1"),
			},
		},
		Url: &modelpb.URL{
			Original: "original",
			Scheme:   "scheme",
			Full:     "full",
			Domain:   "doain",
			Path:     "path",
			Query:    "query",
			Fragment: "fragment",
			Port:     443,
		},
		Log: &modelpb.Log{
			Level:  "level",
			Logger: "logger",
			Origin: &modelpb.LogOrigin{
				FunctionName: "functionname",
				File: &modelpb.LogOriginFile{
					Name: "name",
					Line: 1,
				},
			},
		},
		Source: &modelpb.Source{
			Ip: modelpb.MustParseIP("127.0.0.1"),
			Nat: &modelpb.NAT{
				Ip: modelpb.MustParseIP("127.0.0.2"),
			},
			Domain: "domain",
			Port:   443,
		},
		Client: &modelpb.Client{
			Ip:     modelpb.MustParseIP("127.0.0.1"),
			Domain: "example.com",
			Port:   443,
		},
		ChildIds: []string{"id"},
		Destination: &modelpb.Destination{
			Address: "127.0.0.1",
			Port:    443,
		},
		Session: &modelpb.Session{
			Id:       "id",
			Sequence: 1,
		},
		Process: &modelpb.Process{
			Ppid: 1,
			Thread: &modelpb.ProcessThread{
				Name: "name",
				Id:   2,
			},
			Title:       "title",
			CommandLine: "commandline",
			Executable:  "executable",
			Argv:        []string{"argv"},
			Pid:         3,
		},
		Event: &modelpb.Event{
			Outcome:  "outcome",
			Action:   "action",
			Dataset:  "dataset",
			Kind:     "kind",
			Category: "category",
			Type:     "type",
			SuccessCount: &modelpb.SummaryMetric{
				Count: 1,
				Sum:   2,
			},
			Duration: uint64(3 * time.Second),
			Severity: 4,
		},
	}
}
