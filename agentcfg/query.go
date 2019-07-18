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

package agentcfg

const (
	// ServiceName keyword
	ServiceName = "service.name"
	// ServiceEnv keyword
	ServiceEnv = "service.environment"
)

// Query represents an URL body or query params for agent configuration
type Query struct {
	Service Service `json:"service"`
}

// Service holds supported attributes for querying configuration
type Service struct {
	Name        string `json:"name"`
	Environment string `json:"environment,omitempty"`
}

// NewQuery creates a Query struct
func NewQuery(name, env string) Query {
	return Query{Service{name, env}}
}

// ID returns the unique id for the query
func (q Query) ID() string {
	if q.Service.Environment == "" {
		return q.Service.Name
	}
	return q.Service.Name + "_" + q.Service.Environment
}
