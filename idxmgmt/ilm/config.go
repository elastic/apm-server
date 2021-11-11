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
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/beat"
	libcommon "github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/fmtstr"

	"github.com/elastic/apm-server/idxmgmt/common"
)

const (
	defaultPolicyName = "apm-rollover-30-days"
)

//Config holds information about ILM mode and whether or not the server should manage the setup
type Config struct {
	Enabled bool  `config:"enabled"`
	Setup   Setup `config:"setup"`
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
	EventType   string `config:"event_type"`
	PolicyName  string `config:"policy_name"`
	IndexSuffix string `config:"index_suffix"`
	Index       string `config:"-"`
}

// Policy contains information about an ILM policy and the name
type Policy struct {
	Name string                 `config:"name"`
	Body map[string]interface{} `config:"policy"`
}

// NewConfig extracts given configuration and merges with default configuration.
func NewConfig(info beat.Info, cfg *libcommon.Config) (Config, error) {
	config := Config{
		Enabled: true,
		Setup:   Setup{Enabled: true, RequirePolicy: true, Mappings: defaultMappings()},
	}
	if cfg != nil {
		if err := cfg.Unpack(&config); err != nil {
			return Config{}, err
		}
	}
	if len(config.Setup.Policies) == 0 {
		// https://github.com/elastic/go-ucfg/issues/167 describes a bug in go-ucfg
		// that panics when trying to unpack an empty configuration for an attribute
		// of type map[string]interface{} into a variable with existing values for the map.
		// This requires some workaround in merging configured policies with default policies
		// TODO(simitt): when the bug is fixed
		// - move the validation part into a `Validate` method
		// - remove the extra handling for `defaultPolicies` and add to defaultConfig instead.
		config.Setup.Policies = defaultPolicies()
	}
	// replace variable rollover_alias parts with beat information if available
	// otherwise fail as the full alias needs to be known during setup.
	for et, m := range config.Setup.Mappings {
		idx, err := applyStaticFmtstr(info, m.Index)
		if err != nil {
			return Config{}, errors.Wrap(err, "variable part of index suffix cannot be resolved")
		}
		m.Index = strings.ToLower(idx)
		config.Setup.Mappings[et] = m
		if config.Setup.RequirePolicy {
			continue
		}
		if _, ok := config.Setup.Policies[m.PolicyName]; !ok {
			// if require_policy=false and policy does not exist, add it with an empty body
			config.Setup.Policies[m.PolicyName] = Policy{Name: m.PolicyName}
		}
	}
	return config, validate(&config)
}

func (c *Config) SelectorConfig() (*libcommon.Config, error) {
	indicesCfg, err := libcommon.NewConfigFrom(c.conditionalIndices())
	if err != nil {
		return nil, err
	}
	var idcsCfg = libcommon.NewConfig()
	idcsCfg.SetString("index", -1, common.FallbackIndex)
	idcsCfg.SetChild("indices", -1, indicesCfg)
	return idcsCfg, nil
}

func (m *Mappings) Unpack(cfg *libcommon.Config) error {
	var mappings []Mapping
	if err := cfg.Unpack(&mappings); err != nil {
		return err
	}
	for _, mapping := range mappings {
		if existing, ok := (*m)[mapping.EventType]; ok {
			if mapping.PolicyName == "" {
				mapping.PolicyName = existing.PolicyName
			}
			mapping.Index = existing.Index
		}
		if mapping.IndexSuffix != "" {
			mapping.Index = fmt.Sprintf("%s-%s", mapping.Index, mapping.IndexSuffix)
		}
		(*m)[mapping.EventType] = mapping
	}
	return nil
}

func (p *Policies) Unpack(cfg *libcommon.Config) error {
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
		if !c.Setup.RequirePolicy {
			// `require_policy=false` indicates that policies are set up outside
			// the APM Server, therefore do not throw an error here.
			return nil
		}
		if _, ok := c.Setup.Policies[m.PolicyName]; !ok {
			return errors.Errorf(""+
				"policy '%s' not configured for ILM setup, "+
				"set `apm-server.ilm.require_policy: false` to disable verification",
				m.PolicyName,
			)
		}
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

func applyStaticFmtstr(info beat.Info, s string) (string, error) {
	fmt, err := fmtstr.CompileEvent(s)
	if err != nil {
		return "", err
	}
	return fmt.Run(&beat.Event{
		Fields: libcommon.MapStr{
			// beat object was left in for backward compatibility reason for older configs.
			"beat": libcommon.MapStr{
				"name":    info.Beat,
				"version": info.Version,
			},
			"observer": libcommon.MapStr{
				"name":    info.Beat,
				"version": info.Version,
			},
		},
		Timestamp: time.Now(),
	})
}

func defaultMappings() map[string]Mapping {
	m := map[string]Mapping{}
	for _, et := range common.EventTypes {
		m[et] = Mapping{EventType: et, PolicyName: defaultPolicyName,
			Index: fmt.Sprintf("%s-%s", common.APMPrefix, et)}
	}
	return m
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
					},
				},
			},
		},
	}
}

func (c *Config) conditionalIndices() []map[string]interface{} {
	conditions := []map[string]interface{}{
		common.ConditionalOnboardingIndex(),
		common.ConditionalSourcemapIndex(),
	}
	for _, m := range c.Setup.Mappings {
		conditions = append(conditions, common.Condition(m.EventType, m.Index))
	}
	return conditions
}
