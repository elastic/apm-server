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

package ilm

import (
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/common"
	libilm "github.com/elastic/beats/v7/libbeat/idxmgmt/ilm"
)

const (
	defaultPolicyName = "apm-rollover-30-days"
)

//Config holds information about ILM mode and whether or not the server should manage the setup
type Config struct {
	Mode  libilm.Mode `config:"enabled"`
	Setup Setup       `config:"setup"`
}

//Setup holds information about how to setup ILM
type Setup struct {
	Enabled       bool     `config:"enabled"`
	Overwrite     bool     `config:"overwrite"`
	RequirePolicy bool     `config:"require_policy"`
	Mappings      Mappings `config:"mapping"`
	Policies      Policies `config:"policies"`
}

type Mappings map[string]Mapping
type Policies map[string]Policy

//Mapping binds together an ILM policy's name and body with an event type
type Mapping struct {
	EventType     string `config:"event_type"`
	PolicyName    string `config:"policy_name"`
	RolloverAlias string `config:"rollover_alias"`
}

// Policy contains information about an ILM policy and the name
type Policy struct {
	Name string                 `config:"name"`
	Body map[string]interface{} `config:"policy"`
}

// NewConfig extracts given configuration and merges with default configuration
// https://github.com/elastic/go-ucfg/issues/167 describes a bug in go-ucfg
// that panics when trying to unpack an empty configuration for an attribute
// of type map[string]interface{} into a variable with existing values for the map.
// This requires some workaround in merging configured policies with default policies
// TODO(simitt): when the bug is fixed
// - move the validation part into a `Validate` method
// - remove the extra handling for `defaultPolicies` and add to defaultConfig instead.
func NewConfig(cfg *common.Config) (Config, error) {
	config := Config{Mode: libilm.ModeAuto,
		Setup: Setup{Enabled: true, RequirePolicy: true, Mappings: defaultMappings()}}
	if cfg != nil {
		if err := cfg.Unpack(&config); err != nil {
			return Config{}, err
		}
	}
	if len(config.Setup.Policies) == 0 {
		config.Setup.Policies = defaultPolicies()
	}
	return config, validate(&config)
}

// validate configuration and raise error if unexpected input given
func validate(c *Config) error {
	definedMappings := defaultMappings()
	for _, m := range c.Setup.Mappings {
		if _, ok := definedMappings[m.EventType]; !ok {
			return errors.Errorf("event_type '%s' not supported for ILM setup", m.EventType)
		}
		if m.PolicyName == "" {
			return errors.New("empty policy_name not supported for ILM setup")
		}
		if m.RolloverAlias == "" {
			return errors.New("empty rollover_alias not supported for ILM setup")
		}
		if !c.Setup.RequirePolicy {
			// `require_policy=false` indicates that policies are set up outside
			// the APM Server, therefore do not throw an error here.
			return nil
		}
		if _, ok := c.Setup.Policies[m.PolicyName]; !ok {
			return errors.Errorf("policy '%s' not configured for ILM setup, "+
				"set `apm-server.ilm.require_policy: false` to disable verification", m.PolicyName)
		}
	}
	return nil
}

func (m *Mappings) Unpack(cfg *common.Config) error {
	var mappings []Mapping
	if err := cfg.Unpack(&mappings); err != nil {
		return err
	}
	for _, mapping := range mappings {
		if existing, ok := (*m)[mapping.EventType]; ok {
			if mapping.PolicyName == "" {
				mapping.PolicyName = existing.PolicyName
			}
			if mapping.RolloverAlias == "" {
				mapping.RolloverAlias = existing.RolloverAlias
			}
		}
		(*m)[mapping.EventType] = mapping

	}
	return nil
}

func (p *Policies) Unpack(cfg *common.Config) error {
	// TODO(simitt): remove setting the default policies when
	// https://github.com/elastic/go-ucfg/issues/167 is fixed
	(*p) = defaultPolicies()

	var policies []Policy
	if err := cfg.Unpack(&policies); err != nil {
		return err
	}
	for _, policy := range policies {
		body := preparePolicyBody(policy.Body)
		policy.Body = map[string]interface{}{"policy": body}
		(*p)[policy.Name] = policy
	}
	return nil
}

//preparePolicyBody ensures maps are in the format elasticsearch expects for policy bodies
func preparePolicyBody(m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		if v == nil {
			//ensure nil values are replaced with an empty map,
			// e.g. `delete: {}` instead of `delete: nil`
			m[k] = map[string]interface{}{}
			continue
		} else if val, ok := v.(map[string]interface{}); ok && val != nil {
			m[k] = preparePolicyBody(val)
		}
	}
	return m
}

func defaultMappings() map[string]Mapping {
	return map[string]Mapping{
		"error": {EventType: "error", PolicyName: defaultPolicyName,
			RolloverAlias: "apm-%{[observer.version]}-error"},
		"span": {EventType: "span", PolicyName: defaultPolicyName,
			RolloverAlias: "apm-%{[observer.version]}-span"},
		"transaction": {EventType: "transaction", PolicyName: defaultPolicyName,
			RolloverAlias: "apm-%{[observer.version]}-transaction"},
		"metric": {EventType: "metric", PolicyName: defaultPolicyName,
			RolloverAlias: "apm-%{[observer.version]}-metric"},
		"profile": {EventType: "profile", PolicyName: defaultPolicyName,
			RolloverAlias: "apm-%{[observer.version]}-profile"},
	}
}

func defaultPolicies() map[string]Policy {
	return map[string]Policy{
		defaultPolicyName: {
			Name: defaultPolicyName,
			Body: map[string]interface{}{
				"policy": map[string]interface{}{
					"phases": map[string]interface{}{
						"hot": map[string]interface{}{
							"actions": map[string]interface{}{
								"rollover": map[string]interface{}{
									"max_size": "50gb",
									"max_age":  "30d",
								},
								"set_priority": map[string]interface{}{
									"priority": 100,
								},
							},
						},
						"warm": map[string]interface{}{
							"min_age": "30d",
							"actions": map[string]interface{}{
								"set_priority": map[string]interface{}{
									"priority": 50,
								},
								"readonly": map[string]interface{}{},
							},
						},
					},
				},
			},
		},
	}
}
