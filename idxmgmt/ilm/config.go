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

	"github.com/elastic/beats/libbeat/common"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
)

var (
	errPolicyFmt       = errors.New("input for ILM policy is in wrong format")
	errPolicyNameEmpty = errors.New("empty policy_name not supported for ILM setup")
)

//Config holds information about ILM mode and whether or not the server should manage the setup
type Config struct {
	Mode  libilm.Mode `config:"enabled"`
	Setup Setup       `config:"setup"`
}

//Setup holds information about how to setup ILM
type Setup struct {
	Enabled       bool `config:"enabled"`
	RequirePolicy bool `config:"require_policy"`
	Policies      []EventPolicy
}

//EventPolicy binds together an ILM policy's name and body with an event type
type EventPolicy struct {
	Policy    map[string]interface{}
	EventType string
	Name      string
}

//NewConfig returns an ILM config, where default configuration is merged with user configuration
func NewConfig(inputConfig *common.Config) (Config, error) {
	policyPool := policyPool()
	policyMappings := policyMapping()
	c := defaultConfig()
	if inputConfig != nil {
		if err := inputConfig.Unpack(&c); err != nil {
			return Config{}, err
		}
	}

	// Unpack and process user configuration only if setup.enabled=true
	if inputConfig != nil && c.Setup.Enabled {
		var tmpConfig config
		if err := inputConfig.Unpack(&tmpConfig); err != nil {
			return Config{}, err
		}
		// create a collection of default and configured policies
		for _, policy := range tmpConfig.PolicyPool {
			if policy.Name == "" {
				return Config{}, errPolicyNameEmpty
			}
			policyPool[policy.Name] = policy.Body
		}
		//update policy name per event according to configuration
		for _, entry := range tmpConfig.Mapping {
			if _, ok := policyMappings[entry.Event]; !ok {
				return Config{}, errors.Errorf("event_type '%s' not supported for ILM setup", entry.Event)
			}
			policyMappings[entry.Event] = entry.PolicyName
		}
	}

	var eventPolicies []EventPolicy
	for event, policyName := range policyMappings {
		if policyName == "" {
			return Config{}, errPolicyNameEmpty
		}
		policy, ok := policyPool[policyName]
		if !ok && c.Setup.RequirePolicy {
			return Config{}, errors.Errorf("policy '%s' not configured for ILM setup, "+
				"set `apm-server.ilm.require_policy: false` to disable verification", policyName)
		}
		eventPolicies = append(eventPolicies, EventPolicy{EventType: event, Policy: policy, Name: policyName})
	}
	c.Setup.Policies = eventPolicies
	return c, nil
}

func defaultConfig() Config {
	return Config{Mode: libilm.ModeAuto, Setup: Setup{Enabled: true, RequirePolicy: true}}
}

type config struct {
	Mapping []struct {
		PolicyName string `config:"policy_name"`
		Event      string `config:"event_type"`
	} `config:"setup.mapping"`
	PolicyPool []policy `config:"setup.policies"`
}

type policy struct {
	Name string     `config:"name"`
	Body policyBody `config:"policy"`
}
type policyBody map[string]interface{}

func (p *policyBody) Unpack(i interface{}) error {
	//prepare ensures maps are in the format elasticsearch expects for policy bodies,
	var prepare func(map[string]interface{}) map[string]interface{}
	prepare = func(m map[string]interface{}) map[string]interface{} {
		for k, v := range m {
			if v == nil {
				//ensure nil values are replaced with an empty map,
				// e.g. `delete: {}` instead of `delete: nil`
				m[k] = map[string]interface{}{}
			} else if val, ok := v.(map[string]interface{}); ok && val != nil {
				m[k] = prepare(val)
			}
		}
		return m
	}

	inp, ok := i.(map[string]interface{})
	if !ok {
		return errPolicyFmt
	}
	(*p) = map[string]interface{}{policyStr: prepare(inp)}

	return nil
}
