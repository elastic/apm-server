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

package otel

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"
	semconv "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/common"
)

const (
	AgentNameJaeger = "Jaeger"
)

var (
	serviceNameInvalidRegexp = regexp.MustCompile("[^a-zA-Z0-9 _-]")
)

func translateResourceMetadata(resource pdata.Resource, out *model.APMEvent) {
	var exporterVersion string
	resource.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		switch k {
		// service.*
		case semconv.AttributeServiceName:
			out.Service.Name = cleanServiceName(v.StringVal())
		case semconv.AttributeServiceVersion:
			out.Service.Version = truncate(v.StringVal())
		case semconv.AttributeServiceInstanceID:
			out.Service.Node.Name = truncate(v.StringVal())

		// deployment.*
		case semconv.AttributeDeploymentEnvironment:
			out.Service.Environment = truncate(v.StringVal())

		// telemetry.sdk.*
		case semconv.AttributeTelemetrySDKName:
			out.Agent.Name = truncate(v.StringVal())
		case semconv.AttributeTelemetrySDKVersion:
			out.Agent.Version = truncate(v.StringVal())
		case semconv.AttributeTelemetrySDKLanguage:
			out.Service.Language.Name = truncate(v.StringVal())

		// cloud.*
		case semconv.AttributeCloudProvider:
			out.Cloud.Provider = truncate(v.StringVal())
		case semconv.AttributeCloudAccountID:
			out.Cloud.AccountID = truncate(v.StringVal())
		case semconv.AttributeCloudRegion:
			out.Cloud.Region = truncate(v.StringVal())
		case semconv.AttributeCloudAvailabilityZone:
			out.Cloud.AvailabilityZone = truncate(v.StringVal())
		case semconv.AttributeCloudPlatform:
			out.Cloud.ServiceName = truncate(v.StringVal())

		// container.*
		case semconv.AttributeContainerName:
			out.Container.Name = truncate(v.StringVal())
		case semconv.AttributeContainerID:
			out.Container.ID = truncate(v.StringVal())
		case semconv.AttributeContainerImageName:
			out.Container.ImageName = truncate(v.StringVal())
		case semconv.AttributeContainerImageTag:
			out.Container.ImageTag = truncate(v.StringVal())
		case "container.runtime":
			out.Container.Runtime = truncate(v.StringVal())

		// k8s.*
		case semconv.AttributeK8SNamespaceName:
			out.Kubernetes.Namespace = truncate(v.StringVal())
		case semconv.AttributeK8SNodeName:
			out.Kubernetes.NodeName = truncate(v.StringVal())
		case semconv.AttributeK8SPodName:
			out.Kubernetes.PodName = truncate(v.StringVal())
		case semconv.AttributeK8SPodUID:
			out.Kubernetes.PodUID = truncate(v.StringVal())

		// host.*
		case semconv.AttributeHostName:
			out.Host.Hostname = truncate(v.StringVal())
		case semconv.AttributeHostID:
			out.Host.ID = truncate(v.StringVal())
		case semconv.AttributeHostType:
			out.Host.Type = truncate(v.StringVal())
		case "host.arch":
			out.Host.Architecture = truncate(v.StringVal())

		// process.*
		case semconv.AttributeProcessPID:
			out.Process.Pid = int(v.IntVal())
		case semconv.AttributeProcessCommandLine:
			out.Process.CommandLine = truncate(v.StringVal())
		case semconv.AttributeProcessExecutablePath:
			out.Process.Executable = truncate(v.StringVal())
		case "process.runtime.name":
			out.Service.Runtime.Name = truncate(v.StringVal())
		case "process.runtime.version":
			out.Service.Runtime.Version = truncate(v.StringVal())

		// os.*
		case semconv.AttributeOSType:
			out.Host.OS.Platform = strings.ToLower(truncate(v.StringVal()))
		case semconv.AttributeOSDescription:
			out.Host.OS.Full = truncate(v.StringVal())

		// Legacy OpenCensus attributes.
		case "opencensus.exporterversion":
			exporterVersion = v.StringVal()

		default:
			if out.Labels == nil {
				out.Labels = make(common.MapStr)
			}
			out.Labels[replaceDots(k)] = ifaceAttributeValue(v)
		}
		return true
	})

	// https://www.elastic.co/guide/en/ecs/current/ecs-os.html#field-os-type:
	//
	// "One of these following values should be used (lowercase): linux, macos, unix, windows.
	// If the OS you’re dealing with is not in the list, the field should not be populated."
	switch out.Host.OS.Platform {
	case "windows", "linux":
		out.Host.OS.Type = out.Host.OS.Platform
	case "darwin":
		out.Host.OS.Type = "macos"
	case "aix", "hpux", "solaris":
		out.Host.OS.Type = "unix"
	}

	if strings.HasPrefix(exporterVersion, "Jaeger") {
		// version is of format `Jaeger-<agentlanguage>-<version>`, e.g. `Jaeger-Go-2.20.0`
		const nVersionParts = 3
		versionParts := strings.SplitN(exporterVersion, "-", nVersionParts)
		if out.Service.Language.Name == "" && len(versionParts) == nVersionParts {
			out.Service.Language.Name = versionParts[1]
		}
		if v := versionParts[len(versionParts)-1]; v != "" {
			out.Agent.Version = v
		}
		out.Agent.Name = AgentNameJaeger

		// Translate known Jaeger labels.
		if clientUUID, ok := out.Labels["client-uuid"].(string); ok {
			out.Agent.EphemeralID = clientUUID
			delete(out.Labels, "client-uuid")
		}
		if systemIP, ok := out.Labels["ip"].(string); ok {
			out.Host.IP = net.ParseIP(systemIP)
			delete(out.Labels, "ip")
		}
	}

	if out.Service.Name == "" {
		// service.name is a required field.
		out.Service.Name = "unknown"
	}
	if out.Agent.Name == "" {
		// agent.name is a required field.
		out.Agent.Name = "otlp"
	}
	if out.Agent.Version == "" {
		// agent.version is a required field.
		out.Agent.Version = "unknown"
	}
	if out.Service.Language.Name != "" {
		out.Agent.Name = fmt.Sprintf("%s/%s", out.Agent.Name, out.Service.Language.Name)
	} else {
		out.Service.Language.Name = "unknown"
	}
}

func cleanServiceName(name string) string {
	return serviceNameInvalidRegexp.ReplaceAllString(truncate(name), "_")
}

func ifaceAttributeValue(v pdata.AttributeValue) interface{} {
	switch v.Type() {
	case pdata.AttributeValueTypeString:
		return truncate(v.StringVal())
	case pdata.AttributeValueTypeInt:
		return v.IntVal()
	case pdata.AttributeValueTypeDouble:
		return v.DoubleVal()
	case pdata.AttributeValueTypeBool:
		return v.BoolVal()
	case pdata.AttributeValueTypeArray:
		return ifaceAnyValueArray(v.ArrayVal())
	}
	return nil
}

func ifaceAnyValueArray(array pdata.AnyValueArray) []interface{} {
	values := make([]interface{}, array.Len())
	for i := range values {
		values[i] = ifaceAttributeValue(array.At(i))
	}
	return values
}

// initEventLabels initializes an event-specific label map, either making a copy
// of commonLabels if it is non-nil, or otherwise creating a new map.
func initEventLabels(commonLabels common.MapStr) common.MapStr {
	return commonLabels.Clone()
}
