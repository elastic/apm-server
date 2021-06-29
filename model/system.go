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
	// DetectedHostname holds the detected hostname.
	//
	// This will be written to the event as "host.hostname".
	//
	// TODO(axw) rename this to Hostname.
	DetectedHostname string

	// ConfiguredHostname holds the user-defined or detected hostname.
	//
	// If defined, this will be written to the event as "host.name".
	//
	// TODO(axw) rename this to Name.
	ConfiguredHostname string

	// ID holds a unique ID for the host.
	ID string

	Architecture string
	Platform     string
	FullPlatform string // Full operating system name, including version
	OSType       string
	Type         string // host type, e.g. cloud instance machine type
	IP           net.IP

	Container  Container
	Kubernetes Kubernetes
	Network    Network
}

func (s *System) fields() common.MapStr {
	if s == nil {
		return nil
	}
	var system mapStr
	system.maybeSetString("hostname", s.DetectedHostname)
	system.maybeSetString("name", s.ConfiguredHostname)
	system.maybeSetString("architecture", s.Architecture)
	system.maybeSetString("type", s.Type)

	var os mapStr
	os.maybeSetString("platform", s.Platform)
	os.maybeSetString("full", s.FullPlatform)
	os.maybeSetString("type", s.OSType)
	system.maybeSetMapStr("os", common.MapStr(os))

	if s.IP != nil {
		system.set("ip", s.IP.String())
	}
	return common.MapStr(system)
}
