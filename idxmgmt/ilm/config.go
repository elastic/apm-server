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

//Config holds information about ILM mode, overwriting and policies
type Config struct {
	Mode          libilm.Mode `config:"enabled"`
	Overwrite     bool        `config:"overwrite"`
	RequirePolicy bool        `config:"require_policy"`
	Policies      []EventPolicy
}

//EventPolicy binds together an ILM policy's name and body with an event type
type EventPolicy struct {
	Policy    map[string]interface{}
	EventType string
	Name      string
}

//Enabled indicates whether or not ILM should be enabled
func (c *Config) Enabled() bool {
	return c.Mode != libilm.ModeDisabled
}

//NewConfig returns an ILM config, where default configuration is merged with user configuration
func NewConfig(inputConfig *common.Config) (Config, error) {
	policies := policyPool()
	policyMappings := policyMapping()
	c := defaultConfig()
	if inputConfig != nil {
		if err := inputConfig.Unpack(&c); err != nil {
			return Config{}, err
		}

		var tmpConfig config
		if err := inputConfig.Unpack(&tmpConfig); err != nil {
			return Config{}, err
		}
		// create a collection of default and configured policies
		for name, policy := range tmpConfig.Policies {
			policies[name] = policy
		}
		//update policy name per event according to configuration
		for _, entry := range tmpConfig.Mapping {
			if _, ok := policyMappings[entry.Event]; !ok {
				return c, errors.Errorf("event_type '%s' not supported for ILM setup", entry.Event)
			}
			policyMappings[entry.Event] = entry.Policy
		}
	}

	for event, policyName := range policyMappings {
		policy, ok := policies[policyName]
		if !ok {
			if c.RequirePolicy {
				return Config{}, errors.Errorf("policy '%s' not configured for ILM setup", policyName)
			}
			policy = nil
		}
		c.Policies = append(c.Policies, EventPolicy{EventType: event, Policy: policy, Name: policyName})
	}
	return c, nil
}

func defaultConfig() Config {
	return Config{Mode: libilm.ModeAuto, Overwrite: false, RequirePolicy: true}
}

type config struct {
	Mapping []struct {
		Policy string `config:"policy"`
		Event  string `config:"event_type"`
	} `config:"mapping"`
	Policies policies `config:"policies"`
}

type policies map[string]policy
type policy map[string]interface{}

var errPolicyFmt = errors.New("input for ILM policies is in wrong format")

func (p *policies) Unpack(i interface{}) error {
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
	*p = policies{}

	for k, v := range inp {
		inpP, ok := v.(map[string]interface{})
		if !ok {
			return errPolicyFmt
		}
		(*p)[k] = prepare(inpP)
	}
	return nil
}
