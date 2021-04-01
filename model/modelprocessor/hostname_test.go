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

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

func TestSetSystemHostname(t *testing.T) {
	withConfiguredHostname := model.Metadata{
		System: model.System{
			ConfiguredHostname: "configured_hostname",
			DetectedHostname:   "detected_hostname",
		},
	}
	withDetectedHostname := model.Metadata{
		System: model.System{
			DetectedHostname: "detected_hostname",
		},
	}
	withKubernetesPodName := withDetectedHostname
	withKubernetesPodName.System.Kubernetes.PodName = "kubernetes.pod.name"
	withKubernetesNodeName := withKubernetesPodName
	withKubernetesNodeName.System.Kubernetes.NodeName = "kubernetes.node.name"

	processor := modelprocessor.SetSystemHostname{}

	testProcessBatchMetadata(t, processor, withConfiguredHostname, withConfiguredHostname) // unchanged
	testProcessBatchMetadata(t, processor, withDetectedHostname,
		metadataWithConfiguredHostname(
			metadataWithDetectedHostname(withDetectedHostname, "detected_hostname"),
			"detected_hostname",
		),
	)
	testProcessBatchMetadata(t, processor, withKubernetesPodName,
		metadataWithDetectedHostname(withKubernetesPodName, ""),
	)
	testProcessBatchMetadata(t, processor, withKubernetesNodeName,
		metadataWithConfiguredHostname(
			metadataWithDetectedHostname(withKubernetesNodeName, "kubernetes.node.name"),
			"kubernetes.node.name",
		),
	)
}

func metadataWithDetectedHostname(in model.Metadata, detectedHostname string) model.Metadata {
	in.System.DetectedHostname = detectedHostname
	return in
}

func metadataWithConfiguredHostname(in model.Metadata, configuredHostname string) model.Metadata {
	in.System.ConfiguredHostname = configuredHostname
	return in
}
