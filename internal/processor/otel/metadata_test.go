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

package otel_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/elastic/apm-data/model"
)

func TestResourceConventions(t *testing.T) {
	defaultAgent := model.Agent{Name: "otlp", Version: "unknown"}
	defaultService := model.Service{
		Name:     "unknown",
		Language: model.Language{Name: "unknown"},
	}

	for name, test := range map[string]struct {
		attrs    map[string]interface{}
		expected model.APMEvent
	}{
		"empty": {
			attrs:    nil,
			expected: model.APMEvent{Agent: defaultAgent, Service: defaultService},
		},
		"service": {
			attrs: map[string]interface{}{
				"service.name":           "service_name",
				"service.version":        "service_version",
				"deployment.environment": "service_environment",
			},
			expected: model.APMEvent{
				Agent: model.Agent{Name: "otlp", Version: "unknown"},
				Service: model.Service{
					Name:        "service_name",
					Version:     "service_version",
					Environment: "service_environment",
					Language:    model.Language{Name: "unknown"},
				},
			},
		},
		"agent": {
			attrs: map[string]interface{}{
				"telemetry.sdk.name":     "sdk_name",
				"telemetry.sdk.version":  "sdk_version",
				"telemetry.sdk.language": "language_name",
			},
			expected: model.APMEvent{
				Agent: model.Agent{Name: "sdk_name/language_name", Version: "sdk_version"},
				Service: model.Service{
					Name:     "unknown",
					Language: model.Language{Name: "language_name"},
				},
			},
		},
		"runtime": {
			attrs: map[string]interface{}{
				"process.runtime.name":    "runtime_name",
				"process.runtime.version": "runtime_version",
			},
			expected: model.APMEvent{
				Agent: model.Agent{Name: "otlp", Version: "unknown"},
				Service: model.Service{
					Name:     "unknown",
					Language: model.Language{Name: "unknown"},
					Runtime: model.Runtime{
						Name:    "runtime_name",
						Version: "runtime_version",
					},
				},
			},
		},
		"cloud": {
			attrs: map[string]interface{}{
				"cloud.provider":          "provider_name",
				"cloud.region":            "region_name",
				"cloud.account.id":        "account_id",
				"cloud.availability_zone": "availability_zone",
				"cloud.platform":          "platform_name",
			},
			expected: model.APMEvent{
				Agent:   defaultAgent,
				Service: defaultService,
				Cloud: model.Cloud{
					Provider:         "provider_name",
					Region:           "region_name",
					AccountID:        "account_id",
					AvailabilityZone: "availability_zone",
					ServiceName:      "platform_name",
				},
			},
		},
		"container": {
			attrs: map[string]interface{}{
				"container.name":       "container_name",
				"container.id":         "container_id",
				"container.image.name": "container_image_name",
				"container.image.tag":  "container_image_tag",
				"container.runtime":    "container_runtime",
			},
			expected: model.APMEvent{
				Agent:   defaultAgent,
				Service: defaultService,
				Container: model.Container{
					Name:      "container_name",
					ID:        "container_id",
					Runtime:   "container_runtime",
					ImageName: "container_image_name",
					ImageTag:  "container_image_tag",
				},
			},
		},
		"kubernetes": {
			attrs: map[string]interface{}{
				"k8s.namespace.name": "kubernetes_namespace",
				"k8s.node.name":      "kubernetes_node_name",
				"k8s.pod.name":       "kubernetes_pod_name",
				"k8s.pod.uid":        "kubernetes_pod_uid",
			},
			expected: model.APMEvent{
				Agent:   defaultAgent,
				Service: defaultService,
				Kubernetes: model.Kubernetes{
					Namespace: "kubernetes_namespace",
					NodeName:  "kubernetes_node_name",
					PodName:   "kubernetes_pod_name",
					PodUID:    "kubernetes_pod_uid",
				},
			},
		},
		"host": {
			attrs: map[string]interface{}{
				"host.name": "host_name",
				"host.id":   "host_id",
				"host.type": "host_type",
				"host.arch": "host_arch",
			},
			expected: model.APMEvent{
				Agent:   defaultAgent,
				Service: defaultService,
				Host: model.Host{
					Hostname:     "host_name",
					ID:           "host_id",
					Type:         "host_type",
					Architecture: "host_arch",
				},
			},
		},
		"device": {
			attrs: map[string]interface{}{
				"device.id":               "device_id",
				"device.model.identifier": "device_model_identifier",
				"device.model.name":       "device_model_name",
				"device.manufacturer":     "device_manufacturer",
			},
			expected: model.APMEvent{
				Agent:   defaultAgent,
				Service: defaultService,
				Device: model.Device{
					ID: "device_id",
					Model: model.DeviceModel{
						Identifier: "device_model_identifier",
						Name:       "device_model_name",
					},
					Manufacturer: "device_manufacturer",
				},
			},
		},
		"process": {
			attrs: map[string]interface{}{
				"process.pid":             123,
				"process.command_line":    "command_line",
				"process.executable.path": "executable_path",
			},
			expected: model.APMEvent{
				Agent:   defaultAgent,
				Service: defaultService,
				Process: model.Process{
					Pid:         123,
					CommandLine: "command_line",
					Executable:  "executable_path",
				},
			},
		},
		"os": {
			attrs: map[string]interface{}{
				"os.name":        "macOS",
				"os.version":     "10.14.6",
				"os.type":        "DARWIN",
				"os.description": "Mac OS Mojave",
			},
			expected: model.APMEvent{
				Agent:   defaultAgent,
				Service: defaultService,
				Host: model.Host{
					OS: model.OS{
						Name:     "macOS",
						Version:  "10.14.6",
						Platform: "darwin",
						Type:     "macos",
						Full:     "Mac OS Mojave",
					},
				},
			},
		},
		"os ios": {
			attrs: map[string]interface{}{
				"os.name":        "iOS",
				"os.version":     "15.6",
				"os.type":        "DARWIN",
				"os.description": "iOS 15.6",
			},
			expected: model.APMEvent{
				Agent:   defaultAgent,
				Service: defaultService,
				Host: model.Host{
					OS: model.OS{
						Name:     "iOS",
						Version:  "15.6",
						Platform: "darwin",
						Type:     "ios",
						Full:     "iOS 15.6",
					},
				},
			},
		},
		"os android": {
			attrs: map[string]interface{}{
				"os.name":        "Android",
				"os.version":     "13",
				"os.type":        "linux",
				"os.description": "Android 13",
			},
			expected: model.APMEvent{
				Agent:   defaultAgent,
				Service: defaultService,
				Host: model.Host{
					OS: model.OS{
						Name:     "Android",
						Version:  "13",
						Platform: "linux",
						Type:     "android",
						Full:     "Android 13",
					},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			meta := transformResourceMetadata(t, test.attrs)
			assert.Equal(t, test.expected, meta)
		})
	}
}

func TestResourceLabels(t *testing.T) {
	metadata := transformResourceMetadata(t, map[string]interface{}{
		"string_array": []interface{}{"abc", "def"},
		"int_array":    []interface{}{123, 456},
	})
	assert.Equal(t, model.Labels{
		"string_array": {Global: true, Values: []string{"abc", "def"}},
	}, metadata.Labels)
	assert.Equal(t, model.NumericLabels{
		"int_array": {Global: true, Values: []float64{123, 456}},
	}, metadata.NumericLabels)
}

func transformResourceMetadata(t *testing.T, resourceAttrs map[string]interface{}) model.APMEvent {
	traces, spans := newTracesSpans()
	//attrMap := pcommon.NewMap()
	//attrMap.FromRaw(resourceAttrs)
	//attrMap.CopyTo(traces.ResourceSpans().At(0).Resource().Attributes())
	traces.ResourceSpans().At(0).Resource().Attributes().FromRaw(resourceAttrs)
	otelSpan := spans.Spans().AppendEmpty()
	otelSpan.SetTraceID(pcommon.TraceID{1})
	otelSpan.SetSpanID(pcommon.SpanID{2})
	events := transformTraces(t, traces)
	events[0].Transaction = nil
	events[0].Trace = model.Trace{}
	events[0].Event.Outcome = ""
	events[0].Timestamp = time.Time{}
	events[0].Processor = model.Processor{}
	return events[0]
}
