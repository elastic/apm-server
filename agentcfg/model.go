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
	// EtagSentinel is a value to return back to agents when Kibana doesn't have any configuration
	EtagSentinel = "-"
)

var (
	// UnrestrictedSettings are settings considered safe to be returned to all requesters,
	// including unauthenticated ones such as RUM.
	UnrestrictedSettings = map[string]bool{"transaction_sample_rate": true}
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

	// Etag should be set to the Etag of a previous agent config query result.
	// When the query is processed by the receiver a new Etag is calculated
	// for the query result. If Etags from the query and the query result match,
	// it indicates that the exact same query response has already been delivered.
	Etag string `json:"etag"`

	// MarkAsAppliedByAgent can be used to signal to the receiver that the response to this
	// query can be considered to have been applied immediately. When building queries for Elastic APM
	// agent requests the Etag should be set, instead of the AppliedByAgent setting.
	// Use this flag when building queries for third party integrations,
	// such as Jaeger, that do not send an Etag in their request.
	MarkAsAppliedByAgent *bool `json:"mark_as_applied_by_agent,omitempty"`

	// InsecureAgents holds a set of prefixes for restricting results to those whose
	// agent name matches any of the specified prefixes.
	//
	// If InsecureAgents is non-empty, and any of the prefixes matches the result,
	// then the resulting settings will be filtered down to the subset of settings
	// identified by UnrestrictedSettings. Otherwise, if InsecureAgents is empty,
	// the agent name is ignored and no restrictions are applied.
	InsecureAgents []string `json:"-"`
}

func (q Query) id() string {
	return q.Service.Name + q.Service.Environment
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
	return Result{Source: Source{Settings: Settings{}, Etag: EtagSentinel}}
}

func newResult(b []byte, err error) (Result, error) {
	r := zeroResult()
	if err == nil && len(b) > 0 {
		err = json.Unmarshal(b, &r)
	}
	return r, err
}
