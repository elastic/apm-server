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
	"github.com/elastic/beats/libbeat/common"
)

type System struct {
	Hostname     *string
	Architecture *string
	Platform     *string
	IP           *string

	Container  *Container
	Kubernetes *Kubernetes
}

func DecodeSystem(input interface{}, err error) (*System, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for system")
	}
	decoder := utility.ManualDecoder{}
	system := System{
		Hostname:     decoder.StringPtr(raw, "hostname"),
		Platform:     decoder.StringPtr(raw, "platform"),
		Architecture: decoder.StringPtr(raw, "architecture"),
		IP:           decoder.StringPtr(raw, "ip"),
	}
	if system.Container, err = DecodeContainer(raw["container"], err); err != nil {
		return nil, err
	}
	if system.Kubernetes, err = DecodeKubernetes(raw["kubernetes"], err); err != nil {
		return nil, err
	}
	return &system, decoder.Err
}

func (s *System) fields() common.MapStr {
	if s == nil {
		return nil
	}
	system := common.MapStr{}
	utility.Set(system, "hostname", s.Hostname)
	utility.Set(system, "architecture", s.Architecture)
	if s.Platform != nil {
		utility.Set(system, "os", common.MapStr{"platform": s.Platform})
	}
	if s.IP != nil && *s.IP != "" {
		utility.Set(system, "ip", s.IP)
	}
	return system
}

func (s *System) containerFields() common.MapStr {
	if s == nil {
		return nil
	}
	return s.Container.fields()
}

func (s *System) kubernetesFields() common.MapStr {
	if s == nil {
		return nil
	}
	return s.Kubernetes.fields()
}
