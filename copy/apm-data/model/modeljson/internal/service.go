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

type Service struct {
	Node        *ServiceNode   `json:"node,omitempty"`
	Language    *Language      `json:"language,omitempty"`
	Runtime     *Runtime       `json:"runtime,omitempty"`
	Framework   *Framework     `json:"framework,omitempty"`
	Origin      *ServiceOrigin `json:"origin,omitempty"`
	Target      *ServiceTarget `json:"target,omitempty"`
	Name        string         `json:"name,omitempty"`
	Version     string         `json:"version,omitempty"`
	Environment string         `json:"environment,omitempty"`
}

type ServiceOrigin struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type ServiceTarget struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type"` // intentionally not omitempty
}

type Language struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type Runtime struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type Framework struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type ServiceNode struct {
	Name string `json:"name,omitempty"`
}
