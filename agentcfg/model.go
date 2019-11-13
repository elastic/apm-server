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
)

const (
	// ServiceName keyword
	ServiceName = "service.name"
	// ServiceEnv keyword
	ServiceEnv = "service.environment"
	// Etag / If-None-Match keyword
	Etag = "ifnonematch"
)

var (
	// RumAgent keywords (new and old)
	RumAgent = []string{"rum-js", "base-js"}
	// RumSettings are whitelisted applicable settings for RUM
	RumSettings = []string{"transaction_sample_rate"}
)

// Result models a Kibana response
type Result struct {
	Source Source `json:"_source"`
}

// Source is the Elasticsearch _source
type Source struct {
	Settings Settings `json:"settings"`
	Etag     string   `json:"etag"`
	Agent    string   `json:"agent_name"`
}

// Query represents an URL body or query params for agent configuration
type Query struct {
	Service Service `json:"service"`
	Etag    string  `json:"etag"`
	IsRum   bool    `json:"-"`
}

func (q Query) id() string {
	return q.Service.Name + q.Service.Environment
}

// NewQuery creates a Query struct
func NewQuery(name, env string) Query {
	return Query{Service: Service{name, env}}
}

// Service holds supported attributes for querying configuration
type Service struct {
	Name        string `json:"name"`
	Environment string `json:"environment,omitempty"`
}

// Settings hold agent configuration
type Settings map[string]string

// UnmarshalJSON overrides default method to convert any JSON type to string
func (s Settings) UnmarshalJSON(b []byte) error {
	in := make(map[string]interface{})
	err := json.Unmarshal(b, &in)
	for k, v := range in {
		s[k] = fmt.Sprintf("%v", v)
	}
	return err
}

func zeroResult() Result {
	return Result{Source: Source{Settings: Settings{}}}
}

func newResult(b []byte, err error) (Result, error) {
	r := zeroResult()
	if err == nil && len(b) > 0 {
		err = json.Unmarshal(b, &r)
	}
	return r, err
}
