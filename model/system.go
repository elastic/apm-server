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

package model

import (
	"net"

	"github.com/elastic/beats/v7/libbeat/common"
)

type System struct {
	DetectedHostname   string
	ConfiguredHostname string
	Architecture       string
	Platform           string
	IP                 net.IP
	Container          Container
	Kubernetes         Kubernetes

	// Memory holds the total system memory, in bytes.
	//
	// This may be recorded for short-lived operations,
	// such as browser requests or serverless function
	// invocations, where the amount of available memory
	// may be pertinent to request performance.
	Memory int

	// CPUCores holds the total number of CPU cores.
	//
	// This may be recorded for short-lived operations,
	// such as browser requests or serverless function
	// invocations, where the number of CPUs may be
	// pertinent to request performance.
	CPUCores int
}

func (s *System) name() string {
	if s.ConfiguredHostname != "" {
		return s.ConfiguredHostname
	}
	return s.Hostname()
}

// Hostname returns the value to store in `host.hostname`.
func (s *System) Hostname() string {
	if s == nil {
		return ""
	}

	// if system.kubernetes.node.name is set in the metadata, set host.hostname in the event to its value
	if s.Kubernetes.NodeName != "" {
		return s.Kubernetes.NodeName
	}

	// If system.kubernetes.* is set, but system.kubernetes.node.name is not, then don't set host.hostname at all.
	// some day this could be a hook to discover the right node name using these values
	if s.Kubernetes.PodName != "" || s.Kubernetes.PodUID != "" || s.Kubernetes.Namespace != "" {
		return ""
	}

	// Otherwise set host.hostname to system.hostname
	return s.DetectedHostname
}

func (s *System) fields() common.MapStr {
	if s == nil {
		return nil
	}
	var system mapStr
	system.maybeSetString("hostname", s.Hostname())
	system.maybeSetString("name", s.name())
	system.maybeSetString("architecture", s.Architecture)
	if s.Platform != "" {
		system.set("os", common.MapStr{"platform": s.Platform})
	}
	if s.IP != nil {
		system.set("ip", s.IP.String())
	}
	if s.Memory > 0 {
		system.set("memory.total", s.Memory)
	}
	if s.CPUCores > 0 {
		system.set("cpu.cores", s.CPUCores)
	}
	return common.MapStr(system)
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
