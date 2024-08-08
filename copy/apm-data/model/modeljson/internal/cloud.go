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

type Cloud struct {
	AvailabilityZone string        `json:"availability_zone,omitempty"`
	Provider         string        `json:"provider,omitempty"`
	Region           string        `json:"region,omitempty"`
	Origin           CloudOrigin   `json:"origin,omitempty"`
	Account          CloudAccount  `json:"account,omitempty"`
	Instance         CloudInstance `json:"instance,omitempty"`
	Machine          CloudMachine  `json:"machine,omitempty"`
	Project          CloudProject  `json:"project,omitempty"`
	Service          CloudService  `json:"service,omitempty"`
}

type CloudOrigin struct {
	Account  CloudAccount `json:"account,omitempty"`
	Provider string       `json:"provider,omitempty"`
	Region   string       `json:"region,omitempty"`
	Service  CloudService `json:"service,omitempty"`
}

func (o *CloudOrigin) isZero() bool {
	return *o == CloudOrigin{}
}

type CloudService struct {
	Name string `json:"name"`
}

func (s *CloudService) isZero() bool {
	return s.Name == ""
}

type CloudAccount struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func (a *CloudAccount) isZero() bool {
	return a.ID == "" && a.Name == ""
}

type CloudInstance struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func (i *CloudInstance) isZero() bool {
	return i.ID == "" && i.Name == ""
}

type CloudMachine struct {
	Type string `json:"type,omitempty"`
}

func (m *CloudMachine) isZero() bool {
	return m.Type == ""
}

type CloudProject struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func (p *CloudProject) isZero() bool {
	return p.ID == "" && p.Name == ""
}
