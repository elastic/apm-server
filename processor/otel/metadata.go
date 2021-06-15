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

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/common"
)

const (
	AgentNameJaeger = "Jaeger"

	// Network attributes are pending approval in the OTel spec, and subject to change:
	// https://github.com/open-telemetry/opentelemetry-specification/issues/1647
	
	AttributeNetworkType        = "net.host.connection_type"
	AttributeNetworkMCC         = "net.host.carrier.mcc"
	AttributeNetworkMNC         = "net.host.carrier.mnc"
	AttributeNetworkCarrierName = "net.host.carrier.name"
	AttributeNetworkICC         = "net.host.carrier.icc"
)

var (
	serviceNameInvalidRegexp = regexp.MustCompile("[^a-zA-Z0-9 _-]")
)

func translateResourceMetadata(resource pdata.Resource, out *model.Metadata) {
	var exporterVersion string
	resource.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		switch k {
		// service.*
		case conventions.AttributeServiceName:
			out.Service.Name = cleanServiceName(v.StringVal())
		case conventions.AttributeServiceVersion:
			out.Service.Version = truncate(v.StringVal())
		case conventions.AttributeServiceInstance:
			out.Service.Node.Name = truncate(v.StringVal())

		// deployment.*
		case conventions.AttributeDeploymentEnvironment:
			out.Service.Environment = truncate(v.StringVal())

		// telemetry.sdk.*
		case conventions.AttributeTelemetrySDKName:
			out.Service.Agent.Name = truncate(v.StringVal())
		case conventions.AttributeTelemetrySDKLanguage:
			out.Service.Language.Name = truncate(v.StringVal())
		case conventions.AttributeTelemetrySDKVersion:
			out.Service.Agent.Version = truncate(v.StringVal())

		// cloud.*
		case conventions.AttributeCloudProvider:
			out.Cloud.Provider = truncate(v.StringVal())
		case conventions.AttributeCloudAccount:
			out.Cloud.AccountID = truncate(v.StringVal())
		case conventions.AttributeCloudRegion:
			out.Cloud.Region = truncate(v.StringVal())
		case conventions.AttributeCloudAvailabilityZone:
			out.Cloud.AvailabilityZone = truncate(v.StringVal())
		case conventions.AttributeCloudPlatform:
			out.Cloud.ServiceName = truncate(v.StringVal())

		// container.*
		case conventions.AttributeContainerName:
			out.System.Container.Name = truncate(v.StringVal())
		case conventions.AttributeContainerID:
			out.System.Container.ID = truncate(v.StringVal())
		case conventions.AttributeContainerImage:
			out.System.Container.ImageName = truncate(v.StringVal())
		case conventions.AttributeContainerTag:
			out.System.Container.ImageTag = truncate(v.StringVal())
		case "container.runtime":
			out.System.Container.Runtime = truncate(v.StringVal())

		// k8s.*
		case conventions.AttributeK8sNamespace:
			out.System.Kubernetes.Namespace = truncate(v.StringVal())
		case conventions.AttributeK8sNodeName:
			out.System.Kubernetes.NodeName = truncate(v.StringVal())
		case conventions.AttributeK8sPod:
			out.System.Kubernetes.PodName = truncate(v.StringVal())
		case conventions.AttributeK8sPodUID:
			out.System.Kubernetes.PodUID = truncate(v.StringVal())

		// network.*
		case AttributeNetworkType:
			out.System.Network.ConnectionType = truncate(v.StringVal())
		case AttributeNetworkCarrierName:
			out.System.Network.Carrier.Name = truncate(v.StringVal())
		case AttributeNetworkMCC:
			out.System.Network.Carrier.MCC = truncate(v.StringVal())
		case AttributeNetworkMNC:
			out.System.Network.Carrier.MNC = truncate(v.StringVal())
		case AttributeNetworkICC:
			out.System.Network.Carrier.ICC = truncate(v.StringVal())

		// host.*
		case conventions.AttributeHostName:
			out.System.DetectedHostname = truncate(v.StringVal())
		case conventions.AttributeHostID:
			out.System.ID = truncate(v.StringVal())
		case conventions.AttributeHostType:
			out.System.Type = truncate(v.StringVal())
		case "host.arch":
			out.System.Architecture = truncate(v.StringVal())

		// process.*
		case conventions.AttributeProcessID:
			out.Process.Pid = int(v.IntVal())
		case conventions.AttributeProcessCommandLine:
			out.Process.CommandLine = truncate(v.StringVal())
		case conventions.AttributeProcessExecutablePath:
			out.Process.Executable = truncate(v.StringVal())
		case "process.runtime.name":
			out.Service.Runtime.Name = truncate(v.StringVal())
		case "process.runtime.version":
			out.Service.Runtime.Version = truncate(v.StringVal())

		// os.*
		case conventions.AttributeOSType:
			out.System.Platform = strings.ToLower(truncate(v.StringVal()))
		case conventions.AttributeOSDescription:
			out.System.FullPlatform = truncate(v.StringVal())

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
	switch out.System.Platform {
	case "windows", "linux":
		out.System.OSType = out.System.Platform
	case "darwin":
		out.System.OSType = "macos"
	case "aix", "hpux", "solaris":
		out.System.OSType = "unix"
	}

	if strings.HasPrefix(exporterVersion, "Jaeger") {
		// version is of format `Jaeger-<agentlanguage>-<version>`, e.g. `Jaeger-Go-2.20.0`
		const nVersionParts = 3
		versionParts := strings.SplitN(exporterVersion, "-", nVersionParts)
		if out.Service.Language.Name == "" && len(versionParts) == nVersionParts {
			out.Service.Language.Name = versionParts[1]
		}
		if v := versionParts[len(versionParts)-1]; v != "" {
			out.Service.Agent.Version = v
		}
		out.Service.Agent.Name = AgentNameJaeger

		// Translate known Jaeger labels.
		if clientUUID, ok := out.Labels["client-uuid"].(string); ok {
			out.Service.Agent.EphemeralID = clientUUID
			delete(out.Labels, "client-uuid")
		}
		if systemIP, ok := out.Labels["ip"].(string); ok {
			out.System.IP = net.ParseIP(systemIP)
			delete(out.Labels, "ip")
		}
	}

	if out.Service.Name == "" {
		// service.name is a required field.
		out.Service.Name = "unknown"
	}
	if out.Service.Agent.Name == "" {
		// service.agent.name is a required field.
		out.Service.Agent.Name = "otlp"
	}
	if out.Service.Agent.Version == "" {
		// service.agent.version is a required field.
		out.Service.Agent.Version = "unknown"
	}
	if out.Service.Language.Name != "" {
		out.Service.Agent.Name = fmt.Sprintf("%s/%s", out.Service.Agent.Name, out.Service.Language.Name)
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
	}
	return nil
}
