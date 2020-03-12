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

package metadata

import (
	"errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/v7/libbeat/common"
)

type Kubernetes struct {
	Namespace *string
	NodeName  *string
	PodName   *string
	PodUID    *string
}

func DecodeKubernetes(input interface{}, err error) (*Kubernetes, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for kubernetes")
	}
	decoder := utility.ManualDecoder{}
	return &Kubernetes{
		Namespace: decoder.StringPtr(raw, "namespace"),
		NodeName:  decoder.StringPtr(raw, "name", "node"),
		PodName:   decoder.StringPtr(raw, "name", "pod"),
		PodUID:    decoder.StringPtr(raw, "uid", "pod"),
	}, decoder.Err
}

func (k *Kubernetes) fields() common.MapStr {
	if k == nil {
		return nil
	}
	kubernetes := common.MapStr{}
	utility.Set(kubernetes, "namespace", k.Namespace)

	node := common.MapStr{}
	utility.Set(node, "name", k.NodeName)

	pod := common.MapStr{}
	utility.Set(pod, "name", k.PodName)
	utility.Set(pod, "uid", k.PodUID)

	utility.Set(kubernetes, "node", node)
	utility.Set(kubernetes, "pod", pod)

	return kubernetes
}
