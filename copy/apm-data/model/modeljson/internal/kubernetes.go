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

type Kubernetes struct {
	Namespace string         `json:"namespace,omitempty"`
	Node      KubernetesNode `json:"node,omitempty"`
	Pod       KubernetesPod  `json:"pod,omitempty"`
}

type KubernetesNode struct {
	Name string `json:"name,omitempty"`
}

func (n *KubernetesNode) isZero() bool {
	return n.Name == ""
}

type KubernetesPod struct {
	Name string `json:"name,omitempty"`
	UID  string `json:"uid,omitempty"`
}

func (p *KubernetesPod) isZero() bool {
	return p.Name == "" && p.UID == ""
}
