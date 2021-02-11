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
)

var (
	serviceNameInvalidRegexp = regexp.MustCompile("[^a-zA-Z0-9 _-]")
)

func translateResourceMetadata(resource pdata.Resource, out *model.Metadata) {
	var exporterVersion string
	resource.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		switch k {
		case conventions.AttributeServiceName:
			out.Service.Name = cleanServiceName(v.StringVal())
		case conventions.AttributeServiceVersion:
			out.Service.Version = truncate(v.StringVal())
		case conventions.AttributeServiceInstance:
			out.Service.Node.Name = truncate(v.StringVal())
		case conventions.AttributeDeploymentEnvironment:
			out.Service.Environment = truncate(v.StringVal())

		case conventions.AttributeTelemetrySDKName:
			out.Service.Agent.Name = truncate(v.StringVal())
		case conventions.AttributeTelemetrySDKLanguage:
			out.Service.Language.Name = truncate(v.StringVal())
		case conventions.AttributeTelemetrySDKVersion:
			out.Service.Agent.Version = truncate(v.StringVal())

		case conventions.AttributeK8sNamespace:
			out.System.Kubernetes.Namespace = truncate(v.StringVal())
		case conventions.AttributeK8sPod:
			out.System.Kubernetes.PodName = truncate(v.StringVal())
		case conventions.AttributeK8sPodUID:
			out.System.Kubernetes.PodUID = truncate(v.StringVal())

		case conventions.AttributeHostName:
			out.System.DetectedHostname = truncate(v.StringVal())

		case conventions.AttributeProcessID:
			out.Process.Pid = int(v.IntVal())

		case conventions.OCAttributeExporterVersion:
			exporterVersion = v.StringVal()

		default:
			if out.Labels == nil {
				out.Labels = make(common.MapStr)
			}
			out.Labels[replaceDots(k)] = ifaceAttributeValue(v)
		}
	})

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
	case pdata.AttributeValueSTRING:
		return truncate(v.StringVal())
	case pdata.AttributeValueINT:
		return v.IntVal()
	case pdata.AttributeValueDOUBLE:
		return v.DoubleVal()
	case pdata.AttributeValueBOOL:
		return v.BoolVal()
	}
	return nil
}
