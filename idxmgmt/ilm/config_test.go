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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
)

func TestConfig_Default(t *testing.T) {
	c, err := NewConfig(nil)
	require.NoError(t, err)
	defaultCfg := Config{
		Mode:          libilm.ModeAuto,
		Overwrite:     false,
		RequirePolicy: true,
		Policies: []Policy{
			{EventType: "error", Policy: policyPool()[rollover1Day], Name: rollover1Day},
			{EventType: "span", Policy: policyPool()[rollover1Day], Name: rollover1Day},
			{EventType: "transaction", Policy: policyPool()[rollover7Days], Name: rollover7Days},
			{EventType: "metric", Policy: policyPool()[rollover7Days], Name: rollover7Days},
		},
	}
	assert.Equal(t, defaultCfg, c)
}

func TestConfig_Mode(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg  m
		mode libilm.Mode
	}{
		"default":  {m{}, libilm.ModeAuto},
		"disabled": {m{"enabled": false}, libilm.ModeDisabled},
		"enabled":  {m{"enabled": true}, libilm.ModeEnabled},
	} {
		t.Run(name, func(t *testing.T) {
			ucfg := common.MustNewConfigFrom(tc.cfg)
			c, err := NewConfig(ucfg)
			require.NoError(t, err)
			assert.Equal(t, tc.mode, c.Mode)
		})
	}
}

func TestConfig_Overwrite(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg       m
		overwrite bool
	}{
		"default": {m{}, false},
		"enabled": {m{"overwrite": true}, true},
	} {
		t.Run(name, func(t *testing.T) {
			ucfg := common.MustNewConfigFrom(tc.cfg)
			c, err := NewConfig(ucfg)
			require.NoError(t, err)
			assert.Equal(t, tc.overwrite, c.Overwrite)
		})
	}
}

func TestConfig_RequirePolicy(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg      m
		required bool
	}{
		"default":      {m{}, true},
		"not required": {m{"require_policy": false}, false},
	} {
		t.Run(name, func(t *testing.T) {
			ucfg := common.MustNewConfigFrom(tc.cfg)
			c, err := NewConfig(ucfg)
			require.NoError(t, err)
			assert.Equal(t, tc.required, c.RequirePolicy)
		})
	}
}

func TestConfig_Policies(t *testing.T) {

	findPolicy := func(p []Policy, name string) map[string]interface{} {
		for _, entry := range p {
			if entry.EventType == name {
				return entry.Policy
			}
		}
		return nil
	}
	deletePolicy := m{"policy": m{"phases": m{"delete": nil}}}

	for name, tc := range map[string]struct {
		event  string
		cfg    m
		policy map[string]interface{}
	}{
		"assign different default policy": {
			"error",
			m{"mapping": []m{{"event_type": "error", "policy": rollover7Days}}},
			policyPool()[rollover7Days]},
		"change default policy": {
			"transaction",
			m{"policies": map[string]m{rollover7Days: deletePolicy}},
			map[string]interface{}{"policy": map[string]interface{}{"phases": map[string]interface{}{"delete": map[string]interface{}{}}}}},
		"assign new policy": {
			"span",
			m{"mapping": []m{{"event_type": "span", "policy": "delete-7-days"}},
				"policies": map[string]m{"delete-7-days": deletePolicy}},
			map[string]interface{}{"policy": map[string]interface{}{"phases": map[string]interface{}{"delete": map[string]interface{}{}}}}},
		"reference missing policy": {
			"span",
			m{"require_policy": false, "mapping": []m{{"event_type": "span", "policy": "foo"}}},
			nil},
	} {
		t.Run(name, func(t *testing.T) {
			ucfg := common.MustNewConfigFrom(tc.cfg)
			c, err := NewConfig(ucfg)
			require.NoError(t, err)
			assert.Equal(t, tc.policy, findPolicy(c.Policies, tc.event))
		})
	}
}

func TestConfig_Invalid(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg    m
		errMsg string
	}{
		"invalid event_type": {m{"mapping": []m{{"event_type": "xyz", "policy": rollover7Days}}}, "event_type 'xyz' not supported"},
		"invalid policy":     {m{"mapping": []m{{"event_type": "span", "policy": "xyz"}}}, "policy 'xyz' not configured"},
	} {
		t.Run(name, func(t *testing.T) {
			ucfg := common.MustNewConfigFrom(tc.cfg)
			_, err := NewConfig(ucfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}

}
