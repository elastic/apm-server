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

package modelprocessor_test

import (
	"testing"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
	"google.golang.org/protobuf/proto"
)

func TestSetHostHostname(t *testing.T) {
	withConfiguredHostname := modelpb.APMEvent{
		Host: &modelpb.Host{
			Name:     "configured_hostname",
			Hostname: "detected_hostname",
		},
	}
	withDetectedHostname := modelpb.APMEvent{
		Host: &modelpb.Host{
			Hostname: "detected_hostname",
		},
	}
	withKubernetesPodName := proto.Clone(&withDetectedHostname).(*modelpb.APMEvent)
	withKubernetesPodName.Kubernetes = &modelpb.Kubernetes{
		PodName: "kubernetes.pod.name",
	}
	withKubernetesNodeName := proto.Clone(withKubernetesPodName).(*modelpb.APMEvent)
	withKubernetesNodeName.Kubernetes.NodeName = "kubernetes.node.name"

	processor := modelprocessor.SetHostHostname{}

	testProcessBatch(t, processor, &withConfiguredHostname, &withConfiguredHostname) // unchanged
	testProcessBatch(t, processor, &withDetectedHostname,
		eventWithHostName(
			eventWithHostHostname(&withDetectedHostname, "detected_hostname"),
			"detected_hostname",
		),
	)
	testProcessBatch(t, processor, withKubernetesPodName,
		eventWithHostHostname(withKubernetesPodName, ""),
	)
	testProcessBatch(t, processor, withKubernetesNodeName,
		eventWithHostName(
			eventWithHostHostname(withKubernetesNodeName, "kubernetes.node.name"),
			"kubernetes.node.name",
		),
	)
}

func eventWithHostHostname(in *modelpb.APMEvent, detectedHostname string) *modelpb.APMEvent {
	in.Host.Hostname = detectedHostname
	return in
}

func eventWithHostName(in *modelpb.APMEvent, configuredHostname string) *modelpb.APMEvent {
	in.Host.Name = configuredHostname
	return in
}
