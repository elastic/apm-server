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
	"net"

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/utility"
)

type System struct {
	DetectedHostname   *string
	ConfiguredHostname *string
	Architecture       *string
	Platform           *string
	IP                 net.IP

	Container  *Container
	Kubernetes *Kubernetes
}

func DecodeSystem(input interface{}, err error) (*System, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for system")
	}
	decoder := utility.ManualDecoder{}
	system := System{
		Platform:     decoder.StringPtr(raw, "platform"),
		Architecture: decoder.StringPtr(raw, "architecture"),
		IP:           decoder.NetIP(raw, "ip"),
	}
	if system.Container, err = DecodeContainer(raw["container"], err); err != nil {
		return nil, err
	}
	if system.Kubernetes, err = DecodeKubernetes(raw["kubernetes"], err); err != nil {
		return nil, err
	}
	detectedHostname := decoder.StringPtr(raw, "detected_hostname")
	configuredHostname := decoder.StringPtr(raw, "configured_hostname")
	if detectedHostname != nil || configuredHostname != nil {
		system.DetectedHostname = detectedHostname
		system.ConfiguredHostname = configuredHostname
	} else {
		system.DetectedHostname = decoder.StringPtr(raw, "hostname")
	}

	return &system, decoder.Err
}

func (s *System) name() *string {
	if s != nil && s.ConfiguredHostname != nil {
		return s.ConfiguredHostname
	}
	return s.hostname()
}

func (s *System) hostname() *string {
	if s == nil {
		return nil
	}

	if s.Kubernetes == nil {
		return s.DetectedHostname
	}

	// if system.kubernetes.node.name is set in the metadata, set host.hostname in the event to its value
	if s.Kubernetes.NodeName != nil {
		return s.Kubernetes.NodeName
	}

	// If system.kubernetes.* is set, but system.kubernetes.node.name is not, then don't set host.hostname at all.
	// some day this could be a hook to discover the right node name using these values
	if s.Kubernetes.PodName != nil || s.Kubernetes.PodUID != nil || s.Kubernetes.Namespace != nil {
		return nil
	}

	// Otherwise set host.hostname to system.hostname
	return s.DetectedHostname
}

func (s *System) fields() common.MapStr {
	if s == nil {
		return nil
	}
	system := common.MapStr{}
	utility.Set(system, "hostname", s.hostname())
	utility.Set(system, "name", s.name())
	utility.Set(system, "architecture", s.Architecture)
	if s.Platform != nil {
		utility.Set(system, "os", common.MapStr{"platform": s.Platform})
	}
	if s.IP != nil {
		utility.Set(system, "ip", s.IP.String())
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
