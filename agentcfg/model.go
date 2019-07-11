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

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// ServiceName keyword
	ServiceName = "service.name"
	// ServiceEnv keyword
	ServiceEnv = "service.environment"
)

// Doc represents an elasticsearch document
type Doc struct {
	ID     string `json:"_id"`
	Source Source `json:"_source"`
}

// Source represents the elasticsearch _source field of a document
type Source struct {
	Settings Settings `json:"settings"`
}

// Settings hold agent configuration
type Settings map[string]string

// UnmarshalJSON overrides default method to convert any JSON type to string
func (s *Settings) UnmarshalJSON(b []byte) error {
	in := make(map[string]interface{})
	out := make(map[string]string)
	err := json.Unmarshal(b, &in)
	for k, v := range in {
		out[k] = fmt.Sprintf("%v", v)
	}
	*s = out
	return err
}

// NewQuery creates a Query struct
func NewQuery(name, env string) Query {
	return Query{Service{name, env}}
}

// Query represents an URL body or query params for agent configuration
type Query struct {
	Service Service `json:"service"`
}

// ID returns the unique id for the query
func (q Query) ID() string {
	var str strings.Builder
	str.WriteString(q.Service.Name)
	if q.Service.Environment != "" {
		str.WriteString("_")
		str.WriteString(q.Service.Environment)
	}
	return str.String()
}

// Service holds supported attributes for querying configuration
type Service struct {
	Name        string `json:"name"`
	Environment string `json:"environment,omitempty"`
}
