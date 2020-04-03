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

package modeldecoder

import (
	"errors"
	"testing"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/stretchr/testify/assert"
)

func TestKubernetesDecode(t *testing.T) {
	namespace, podname, poduid, nodename := "namespace", "podname", "poduid", "podname"
	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		k           *metadata.Kubernetes
	}{
		{input: nil, err: nil, k: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), k: nil},
		{input: "", err: errors.New("invalid type for kubernetes"), k: nil},
		{
			input: map[string]interface{}{
				"namespace": namespace,
				"node":      map[string]interface{}{"name": nodename},
				"pod": map[string]interface{}{
					"uid":  poduid,
					"name": podname,
				},
			},
			err: nil,
			k: &metadata.Kubernetes{
				Namespace: &namespace,
				NodeName:  &nodename,
				PodName:   &podname,
				PodUID:    &poduid,
			},
		},
	} {
		kubernetes, out := DecodeKubernetes(test.input, test.inpErr)
		assert.Equal(t, test.k, kubernetes)
		assert.Equal(t, test.err, out)
	}
}
