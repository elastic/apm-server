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

package otlp_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-data/model/modelpb"
)

func TestResourceConventions(t *testing.T) {
	defaultAgent := modelpb.Agent{Name: "otlp", Version: "unknown"}
	defaultService := modelpb.Service{
		Name:     "unknown",
		Language: &modelpb.Language{Name: "unknown"},
	}

	for name, test := range map[string]struct {
		attrs    map[string]interface{}
		expected *modelpb.APMEvent
	}{
		"empty": {
			attrs:    nil,
			expected: &modelpb.APMEvent{Agent: &defaultAgent, Service: &defaultService},
		},
		"service": {
			attrs: map[string]interface{}{
				"service.name":           "service_name",
				"service.version":        "service_version",
				"service.instance.id":    "service_node_name",
				"deployment.environment": "service_environment",
			},
			expected: &modelpb.APMEvent{
				Agent: &modelpb.Agent{Name: "otlp", Version: "unknown"},
				Service: &modelpb.Service{
					Name:        "service_name",
					Version:     "service_version",
					Environment: "service_environment",
					Node:        &modelpb.ServiceNode{Name: "service_node_name"},
					Language:    &modelpb.Language{Name: "unknown"},
				},
			},
		},
		"agent": {
			attrs: map[string]interface{}{
				"telemetry.sdk.name":     "sdk_name",
				"telemetry.sdk.version":  "sdk_version",
				"telemetry.sdk.language": "language_name",
			},
			expected: &modelpb.APMEvent{
				Agent: &modelpb.Agent{Name: "sdk_name/language_name", Version: "sdk_version"},
				Service: &modelpb.Service{
					Name:     "unknown",
					Language: &modelpb.Language{Name: "language_name"},
				},
			},
		},
		"agent_distro": {
			attrs: map[string]interface{}{
				"telemetry.sdk.name":       "sdk_name",
				"telemetry.sdk.version":    "sdk_version",
				"telemetry.sdk.language":   "language_name",
				"telemetry.distro.name":    "distro_name",
				"telemetry.distro.version": "distro_version",
			},
			expected: &modelpb.APMEvent{
				Agent: &modelpb.Agent{Name: "sdk_name/language_name/distro_name", Version: "distro_version"},
				Service: &modelpb.Service{
					Name:     "unknown",
					Language: &modelpb.Language{Name: "language_name"},
				},
			},
		},
		"agent_distro_no_language": {
			attrs: map[string]interface{}{
				"telemetry.sdk.name":       "sdk_name",
				"telemetry.sdk.version":    "sdk_version",
				"telemetry.distro.name":    "distro_name",
				"telemetry.distro.version": "distro_version",
			},
			expected: &modelpb.APMEvent{
				Agent: &modelpb.Agent{Name: "sdk_name/unknown/distro_name", Version: "distro_version"},
				Service: &modelpb.Service{
					Name:     "unknown",
					Language: &modelpb.Language{Name: "unknown"},
				},
			},
		},
		"agent_distro_no_version": {
			attrs: map[string]interface{}{
				"telemetry.sdk.name":     "sdk_name",
				"telemetry.sdk.version":  "sdk_version",
				"telemetry.sdk.language": "language_name",
				"telemetry.distro.name":  "distro_name",
			},
			expected: &modelpb.APMEvent{
				Agent: &modelpb.Agent{Name: "sdk_name/language_name/distro_name", Version: "unknown"},
				Service: &modelpb.Service{
					Name:     "unknown",
					Language: &modelpb.Language{Name: "language_name"},
				},
			},
		},
		"runtime": {
			attrs: map[string]interface{}{
				"process.runtime.name":    "runtime_name",
				"process.runtime.version": "runtime_version",
			},
			expected: &modelpb.APMEvent{
				Agent: &modelpb.Agent{Name: "otlp", Version: "unknown"},
				Service: &modelpb.Service{
					Name:     "unknown",
					Language: &modelpb.Language{Name: "unknown"},
					Runtime: &modelpb.Runtime{
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
			expected: &modelpb.APMEvent{
				Agent:   &defaultAgent,
				Service: &defaultService,
				Cloud: &modelpb.Cloud{
					Provider:         "provider_name",
					Region:           "region_name",
					AccountId:        "account_id",
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
			expected: &modelpb.APMEvent{
				Agent:   &defaultAgent,
				Service: &defaultService,
				Container: &modelpb.Container{
					Name:      "container_name",
					Id:        "container_id",
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
			expected: &modelpb.APMEvent{
				Agent:   &defaultAgent,
				Service: &defaultService,
				Kubernetes: &modelpb.Kubernetes{
					Namespace: "kubernetes_namespace",
					NodeName:  "kubernetes_node_name",
					PodName:   "kubernetes_pod_name",
					PodUid:    "kubernetes_pod_uid",
				},
			},
		},
		"host": {
			attrs: map[string]interface{}{
				"host.name": "host_name",
				"host.id":   "host_id",
				"host.type": "host_type",
				"host.arch": "host_arch",
				"host.ip":   []interface{}{"10.244.0.1", "172.19.0.2", "fc00:f853:ccd:e793::2"},
			},
			expected: &modelpb.APMEvent{
				Agent:   &defaultAgent,
				Service: &defaultService,
				Host: &modelpb.Host{
					Hostname:     "host_name",
					Id:           "host_id",
					Type:         "host_type",
					Architecture: "host_arch",
					Ip: func() []*modelpb.IP {
						ips := make([]*modelpb.IP, 3)
						ips[0] = modelpb.MustParseIP("10.244.0.1")
						ips[1] = modelpb.MustParseIP("172.19.0.2")
						ips[2] = modelpb.MustParseIP("fc00:f853:ccd:e793::2")
						return ips
					}(),
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
			expected: &modelpb.APMEvent{
				Agent:   &defaultAgent,
				Service: &defaultService,
				Device: &modelpb.Device{
					Id: "device_id",
					Model: &modelpb.DeviceModel{
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
			expected: &modelpb.APMEvent{
				Agent:   &defaultAgent,
				Service: &defaultService,
				Process: &modelpb.Process{
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
			expected: &modelpb.APMEvent{
				Agent:   &defaultAgent,
				Service: &defaultService,
				Host: &modelpb.Host{
					Os: &modelpb.OS{
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
			expected: &modelpb.APMEvent{
				Agent:   &defaultAgent,
				Service: &defaultService,
				Host: &modelpb.Host{
					Os: &modelpb.OS{
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
			expected: &modelpb.APMEvent{
				Agent:   &defaultAgent,
				Service: &defaultService,
				Host: &modelpb.Host{
					Os: &modelpb.OS{
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
			assert.Empty(t, cmp.Diff(test.expected, meta, protocmp.Transform()))
		})
	}
}

func TestResourceLabels(t *testing.T) {
	metadata := transformResourceMetadata(t, map[string]interface{}{
		"string_array": []interface{}{"abc", "def"},
		"int_array":    []interface{}{123, 456},
	})
	assert.Equal(t, modelpb.Labels{
		"string_array": {Global: true, Values: []string{"abc", "def"}},
	}, modelpb.Labels(metadata.Labels))
	assert.Equal(t, modelpb.NumericLabels{
		"int_array": {Global: true, Values: []float64{123, 456}},
	}, modelpb.NumericLabels(metadata.NumericLabels))
}

func transformResourceMetadata(t *testing.T, resourceAttrs map[string]interface{}) *modelpb.APMEvent {
	traces, spans := newTracesSpans()
	traces.ResourceSpans().At(0).Resource().Attributes().FromRaw(resourceAttrs)
	otelSpan := spans.Spans().AppendEmpty()
	otelSpan.SetTraceID(pcommon.TraceID{1})
	otelSpan.SetSpanID(pcommon.SpanID{2})
	events := transformTraces(t, traces)
	(*events)[0].Transaction = nil
	(*events)[0].Span = nil
	(*events)[0].Trace = nil
	(*events)[0].Event = nil
	(*events)[0].Timestamp = 0
	return (*events)[0]
}
