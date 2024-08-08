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

package otlp

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"

	"github.com/elastic/apm-data/model/modelpb"
)

const (
	AgentNameJaeger = "Jaeger"
)

var (
	serviceNameInvalidRegexp = regexp.MustCompile("[^a-zA-Z0-9 _-]")
)

func translateResourceMetadata(resource pcommon.Resource, out *modelpb.APMEvent) {
	var exporterVersion string
	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		switch k {
		// service.*
		case semconv.AttributeServiceName:
			if out.Service == nil {
				out.Service = &modelpb.Service{}
			}
			out.Service.Name = cleanServiceName(v.Str())
		case semconv.AttributeServiceVersion:
			if out.Service == nil {
				out.Service = &modelpb.Service{}
			}
			out.Service.Version = truncate(v.Str())
		case semconv.AttributeServiceInstanceID:
			if out.Service == nil {
				out.Service = &modelpb.Service{}
			}
			if out.Service.Node == nil {
				out.Service.Node = &modelpb.ServiceNode{}
			}
			out.Service.Node.Name = truncate(v.Str())

		// deployment.*
		case semconv.AttributeDeploymentEnvironment:
			if out.Service == nil {
				out.Service = &modelpb.Service{}
			}
			out.Service.Environment = truncate(v.Str())

		// telemetry.sdk.*
		case semconv.AttributeTelemetrySDKName:
			if out.Agent == nil {
				out.Agent = &modelpb.Agent{}
			}
			out.Agent.Name = truncate(v.Str())
		case semconv.AttributeTelemetrySDKVersion:
			if out.Agent == nil {
				out.Agent = &modelpb.Agent{}
			}
			out.Agent.Version = truncate(v.Str())
		case semconv.AttributeTelemetrySDKLanguage:
			if out.Service == nil {
				out.Service = &modelpb.Service{}
			}
			if out.Service.Language == nil {
				out.Service.Language = &modelpb.Language{}
			}
			out.Service.Language.Name = truncate(v.Str())

		// cloud.*
		case semconv.AttributeCloudProvider:
			if out.Cloud == nil {
				out.Cloud = &modelpb.Cloud{}
			}
			out.Cloud.Provider = truncate(v.Str())
		case semconv.AttributeCloudAccountID:
			if out.Cloud == nil {
				out.Cloud = &modelpb.Cloud{}
			}
			out.Cloud.AccountId = truncate(v.Str())
		case semconv.AttributeCloudRegion:
			if out.Cloud == nil {
				out.Cloud = &modelpb.Cloud{}
			}
			out.Cloud.Region = truncate(v.Str())
		case semconv.AttributeCloudAvailabilityZone:
			if out.Cloud == nil {
				out.Cloud = &modelpb.Cloud{}
			}
			out.Cloud.AvailabilityZone = truncate(v.Str())
		case semconv.AttributeCloudPlatform:
			if out.Cloud == nil {
				out.Cloud = &modelpb.Cloud{}
			}
			out.Cloud.ServiceName = truncate(v.Str())

		// container.*
		case semconv.AttributeContainerName:
			if out.Container == nil {
				out.Container = &modelpb.Container{}
			}
			out.Container.Name = truncate(v.Str())
		case semconv.AttributeContainerID:
			if out.Container == nil {
				out.Container = &modelpb.Container{}
			}
			out.Container.Id = truncate(v.Str())
		case semconv.AttributeContainerImageName:
			if out.Container == nil {
				out.Container = &modelpb.Container{}
			}
			out.Container.ImageName = truncate(v.Str())
		case "container.image.tag":
			if out.Container == nil {
				out.Container = &modelpb.Container{}
			}
			out.Container.ImageTag = truncate(v.Str())
		case "container.runtime":
			if out.Container == nil {
				out.Container = &modelpb.Container{}
			}
			out.Container.Runtime = truncate(v.Str())

		// k8s.*
		case semconv.AttributeK8SNamespaceName:
			if out.Kubernetes == nil {
				out.Kubernetes = &modelpb.Kubernetes{}
			}
			out.Kubernetes.Namespace = truncate(v.Str())
		case semconv.AttributeK8SNodeName:
			if out.Kubernetes == nil {
				out.Kubernetes = &modelpb.Kubernetes{}
			}
			out.Kubernetes.NodeName = truncate(v.Str())
		case semconv.AttributeK8SPodName:
			if out.Kubernetes == nil {
				out.Kubernetes = &modelpb.Kubernetes{}
			}
			out.Kubernetes.PodName = truncate(v.Str())
		case semconv.AttributeK8SPodUID:
			if out.Kubernetes == nil {
				out.Kubernetes = &modelpb.Kubernetes{}
			}
			out.Kubernetes.PodUid = truncate(v.Str())

		// host.*
		case semconv.AttributeHostName:
			if out.Host == nil {
				out.Host = &modelpb.Host{}
			}
			out.Host.Hostname = truncate(v.Str())
		case semconv.AttributeHostID:
			if out.Host == nil {
				out.Host = &modelpb.Host{}
			}
			out.Host.Id = truncate(v.Str())
		case semconv.AttributeHostType:
			if out.Host == nil {
				out.Host = &modelpb.Host{}
			}
			out.Host.Type = truncate(v.Str())
		case "host.arch":
			if out.Host == nil {
				out.Host = &modelpb.Host{}
			}
			out.Host.Architecture = truncate(v.Str())
		case semconv.AttributeHostIP:
			if out.Host == nil {
				out.Host = &modelpb.Host{}
			}
			out.Host.Ip = pSliceToType[*modelpb.IP](v.Slice(), func(v pcommon.Value) (*modelpb.IP, bool) {
				ip, err := modelpb.ParseIP(v.Str())
				if err != nil {
					return nil, false
				}
				return ip, true
			})

		// process.*
		case semconv.AttributeProcessPID:
			if out.Process == nil {
				out.Process = &modelpb.Process{}
			}
			out.Process.Pid = uint32(v.Int())
		case semconv.AttributeProcessCommandLine:
			if out.Process == nil {
				out.Process = &modelpb.Process{}
			}
			out.Process.CommandLine = truncate(v.Str())
		case semconv.AttributeProcessExecutablePath:
			if out.Process == nil {
				out.Process = &modelpb.Process{}
			}
			out.Process.Executable = truncate(v.Str())
		case "process.runtime.name":
			if out.Service == nil {
				out.Service = &modelpb.Service{}
			}
			if out.Service.Runtime == nil {
				out.Service.Runtime = &modelpb.Runtime{}
			}
			out.Service.Runtime.Name = truncate(v.Str())
		case "process.runtime.version":
			if out.Service == nil {
				out.Service = &modelpb.Service{}
			}
			if out.Service.Runtime == nil {
				out.Service.Runtime = &modelpb.Runtime{}
			}
			out.Service.Runtime.Version = truncate(v.Str())
		case semconv.AttributeProcessOwner:
			if out.User == nil {
				out.User = &modelpb.User{}
			}
			out.User.Name = truncate(v.Str())

		// os.*
		case semconv.AttributeOSType:
			if out.Host == nil {
				out.Host = &modelpb.Host{}
			}
			if out.Host.Os == nil {
				out.Host.Os = &modelpb.OS{}
			}
			out.Host.Os.Platform = strings.ToLower(truncate(v.Str()))
		case semconv.AttributeOSDescription:
			if out.Host == nil {
				out.Host = &modelpb.Host{}
			}
			if out.Host.Os == nil {
				out.Host.Os = &modelpb.OS{}
			}
			out.Host.Os.Full = truncate(v.Str())
		case semconv.AttributeOSName:
			if out.Host == nil {
				out.Host = &modelpb.Host{}
			}
			if out.Host.Os == nil {
				out.Host.Os = &modelpb.OS{}
			}
			out.Host.Os.Name = truncate(v.Str())
		case semconv.AttributeOSVersion:
			if out.Host == nil {
				out.Host = &modelpb.Host{}
			}
			if out.Host.Os == nil {
				out.Host.Os = &modelpb.OS{}
			}
			out.Host.Os.Version = truncate(v.Str())

		// device.*
		case semconv.AttributeDeviceID:
			if out.Device == nil {
				out.Device = &modelpb.Device{}
			}
			out.Device.Id = truncate(v.Str())
		case semconv.AttributeDeviceModelIdentifier:
			if out.Device == nil {
				out.Device = &modelpb.Device{}
			}
			if out.Device.Model == nil {
				out.Device.Model = &modelpb.DeviceModel{}
			}
			out.Device.Model.Identifier = truncate(v.Str())
		case semconv.AttributeDeviceModelName:
			if out.Device == nil {
				out.Device = &modelpb.Device{}
			}
			if out.Device.Model == nil {
				out.Device.Model = &modelpb.DeviceModel{}
			}
			out.Device.Model.Name = truncate(v.Str())
		case "device.manufacturer":
			if out.Device == nil {
				out.Device = &modelpb.Device{}
			}
			out.Device.Manufacturer = truncate(v.Str())

		// Legacy OpenCensus attributes.
		case "opencensus.exporterversion":
			exporterVersion = v.Str()

		// timestamp attribute to deal with time skew on mobile
		// devices. APM server should drop this field.
		case "telemetry.sdk.elastic_export_timestamp":
			// Do nothing.
		case "telemetry.distro.name":
		case "telemetry.distro.version":
			//distro version & name are handled below and should not end up as labels

		// data_stream.*
		case attributeDataStreamDataset:
			if out.DataStream == nil {
				out.DataStream = &modelpb.DataStream{}
			}
			out.DataStream.Dataset = v.Str()
		case attributeDataStreamNamespace:
			if out.DataStream == nil {
				out.DataStream = &modelpb.DataStream{}
			}
			out.DataStream.Namespace = v.Str()

		default:
			if out.Labels == nil {
				out.Labels = make(modelpb.Labels)
			}
			if out.NumericLabels == nil {
				out.NumericLabels = make(modelpb.NumericLabels)
			}
			setLabel(replaceDots(k), out, ifaceAttributeValue(v))
		}
		return true
	})

	// https://www.elastic.co/guide/en/ecs/current/ecs-os.html#field-os-type:
	//
	// "One of these following values should be used (lowercase): linux, macos, unix, windows.
	// If the OS you’re dealing with is not in the list, the field should not be populated."
	switch out.GetHost().GetOs().GetPlatform() {
	case "windows", "linux":
		out.Host.Os.Type = out.Host.Os.Platform
	case "darwin":
		out.Host.Os.Type = "macos"
	case "aix", "hpux", "solaris":
		out.Host.Os.Type = "unix"
	}

	switch out.GetHost().GetOs().GetName() {
	case "Android":
		out.Host.Os.Type = "android"
	case "iOS":
		out.Host.Os.Type = "ios"
	}

	if strings.HasPrefix(exporterVersion, "Jaeger") {
		// version is of format `Jaeger-<agentlanguage>-<version>`, e.g. `Jaeger-Go-2.20.0`
		const nVersionParts = 3
		versionParts := strings.SplitN(exporterVersion, "-", nVersionParts)
		if out.GetService().GetLanguage().GetName() == "" && len(versionParts) == nVersionParts {
			if out.Service == nil {
				out.Service = &modelpb.Service{}
			}
			if out.Service.Language == nil {
				out.Service.Language = &modelpb.Language{}
			}
			out.Service.Language.Name = versionParts[1]
		}
		if out.Agent == nil {
			out.Agent = &modelpb.Agent{}
		}
		if v := versionParts[len(versionParts)-1]; v != "" {
			out.Agent.Version = v
		}
		out.Agent.Name = AgentNameJaeger

		// Translate known Jaeger labels.
		if clientUUID, ok := out.Labels["client-uuid"]; ok {
			out.Agent.EphemeralId = clientUUID.Value
			delete(out.Labels, "client-uuid")
		}
		if systemIP, ok := out.Labels["ip"]; ok {
			if ip, err := modelpb.ParseIP(systemIP.Value); err == nil {
				out.Host.Ip = []*modelpb.IP{ip}
			}
			delete(out.Labels, "ip")
		}
	}

	if out.GetService().GetName() == "" {
		if out.Service == nil {
			out.Service = &modelpb.Service{}
		}
		// service.name is a required field.
		out.Service.Name = "unknown"
	}
	if out.Agent == nil {
		out.Agent = &modelpb.Agent{}
	}
	if out.GetAgent().GetName() == "" {
		// agent.name is a required field.
		out.Agent.Name = "otlp"
	}
	if out.Agent.Version == "" {
		// agent.version is a required field.
		out.Agent.Version = "unknown"
	}

	distroName, distroNameSet := resource.Attributes().Get("telemetry.distro.name")
	distroVersion, distroVersionSet := resource.Attributes().Get("telemetry.distro.version")

	if distroNameSet && distroName.Str() != "" {
		agentLang := "unknown"
		if out.GetService().GetLanguage().GetName() != "" {
			agentLang = out.GetService().GetLanguage().GetName()
		}

		out.Agent.Name = fmt.Sprintf("%s/%s/%s", out.Agent.Name, agentLang, distroName.Str())

		//we intentionally do not want to fallback to the Otel SDK version if we have a distro name, this would only cause confusion
		out.Agent.Version = "unknown"
		if distroVersionSet && distroVersion.Str() != "" {
			out.Agent.Version = distroVersion.Str()
		}
	} else {
		//distro is not set, use just the language as suffix if present
		if out.GetService().GetLanguage().GetName() != "" {
			out.Agent.Name = fmt.Sprintf("%s/%s", out.Agent.Name, out.GetService().GetLanguage().GetName())
		}
	}

	if out.GetService().GetLanguage().GetName() == "" {
		if out.Service == nil {
			out.Service = &modelpb.Service{}
		}
		if out.Service.Language == nil {
			out.Service.Language = &modelpb.Language{}
		}
		out.Service.Language.Name = "unknown"
	}

	// Set the decoded labels as "global" -- defined at the service level.
	for k, v := range out.Labels {
		v.Global = true
		out.Labels[k] = v
	}
	for k, v := range out.NumericLabels {
		v.Global = true
		out.NumericLabels[k] = v
	}
}

func translateScopeMetadata(scope pcommon.InstrumentationScope, out *modelpb.APMEvent) {
	scope.Attributes().Range(func(k string, v pcommon.Value) bool {
		switch k {
		// data_stream.*
		case attributeDataStreamDataset:
			if out.DataStream == nil {
				out.DataStream = &modelpb.DataStream{}
			}
			out.DataStream.Dataset = v.Str()
		case attributeDataStreamNamespace:
			if out.DataStream == nil {
				out.DataStream = &modelpb.DataStream{}
			}
			out.DataStream.Namespace = v.Str()
		}
		return true
	})
}

func cleanServiceName(name string) string {
	return serviceNameInvalidRegexp.ReplaceAllString(truncate(name), "_")
}

func ifaceAttributeValue(v pcommon.Value) interface{} {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return truncate(v.Str())
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(v.Bool())
	case pcommon.ValueTypeInt:
		return float64(v.Int())
	case pcommon.ValueTypeDouble:
		return v.Double()
	case pcommon.ValueTypeSlice:
		return ifaceAttributeValueSlice(v.Slice())
	}
	return nil
}

func ifaceAttributeValueSlice(slice pcommon.Slice) []interface{} {
	return pSliceToType[interface{}](slice, func(v pcommon.Value) (interface{}, bool) {
		return ifaceAttributeValue(v), true
	})
}

func pSliceToType[T any](slice pcommon.Slice, f func(pcommon.Value) (T, bool)) []T {
	result := make([]T, 0, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		if v, ok := f(slice.At(i)); ok {
			result = append(result, v)
		}
	}
	return result
}

// initEventLabels initializes an event-specific labels from an event.
func initEventLabels(e *modelpb.APMEvent) {
	e.Labels = modelpb.Labels(e.Labels).Clone()
	e.NumericLabels = modelpb.NumericLabels(e.NumericLabels).Clone()
}

func setLabel(key string, event *modelpb.APMEvent, v interface{}) {
	switch v := v.(type) {
	case string:
		modelpb.Labels(event.Labels).Set(key, v)
	case bool:
		modelpb.Labels(event.Labels).Set(key, strconv.FormatBool(v))
	case float64:
		modelpb.NumericLabels(event.NumericLabels).Set(key, v)
	case int64:
		modelpb.NumericLabels(event.NumericLabels).Set(key, float64(v))
	case []interface{}:
		if len(v) == 0 {
			return
		}
		switch v[0].(type) {
		case string:
			value := make([]string, len(v))
			for i := range v {
				value[i] = v[i].(string)
			}
			modelpb.Labels(event.Labels).SetSlice(key, value)
		case float64:
			value := make([]float64, len(v))
			for i := range v {
				value[i] = v[i].(float64)
			}
			modelpb.NumericLabels(event.NumericLabels).SetSlice(key, value)
		}
	}
}
