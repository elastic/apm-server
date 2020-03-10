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

	"github.com/elastic/beats/v7/libbeat/common"
	libilm "github.com/elastic/beats/v7/libbeat/idxmgmt/ilm"
)

func TestConfig_Default(t *testing.T) {
	c, err := NewConfig(nil)
	require.NoError(t, err)
	assert.Equal(t, libilm.ModeAuto, c.Mode)
	assert.True(t, c.Setup.Enabled)
	assert.False(t, c.Setup.Overwrite)
	assert.True(t, c.Setup.RequirePolicy)
	assert.ObjectsAreEqual(defaultPolicies(), c.Setup.Policies)
}

func TestConfig_Mode(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg  map[string]interface{}
		mode libilm.Mode
	}{
		"default":  {map[string]interface{}{}, libilm.ModeAuto},
		"disabled": {map[string]interface{}{"enabled": false}, libilm.ModeDisabled},
		"enabled":  {map[string]interface{}{"enabled": true}, libilm.ModeEnabled},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewConfig(common.MustNewConfigFrom(tc.cfg))
			require.NoError(t, err)
			assert.Equal(t, tc.mode, c.Mode)
		})
	}
}

func TestConfig_SetupEnabled(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg     map[string]interface{}
		enabled bool
	}{
		"enabled":  {map[string]interface{}{"enabled": true}, true},
		"disabled": {map[string]interface{}{"enabled": false}, false},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewConfig(common.MustNewConfigFrom(map[string]interface{}{"setup": tc.cfg}))
			require.NoError(t, err)
			assert.Equal(t, tc.enabled, c.Setup.Enabled)
		})
	}
}

func TestConfig_SetupOverwrite(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg       map[string]interface{}
		overwrite bool
	}{
		"overwrite":        {map[string]interface{}{"overwrite": true}, true},
		"do not overwrite": {map[string]interface{}{"overwrite": false}, false},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewConfig(common.MustNewConfigFrom(map[string]interface{}{"setup": tc.cfg}))
			require.NoError(t, err)
			assert.Equal(t, tc.overwrite, c.Setup.Overwrite)
		})
	}
}

func TestConfig_RequirePolicy(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg      map[string]interface{}
		required bool
	}{
		"default":      {map[string]interface{}{}, true},
		"not required": {map[string]interface{}{"require_policy": false}, false},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewConfig(common.MustNewConfigFrom(map[string]interface{}{"setup": tc.cfg}))
			require.NoError(t, err)
			assert.Equal(t, tc.required, c.Setup.RequirePolicy)
		})
	}
}

func TestConfig_Policies(t *testing.T) {

	findPolicy := func(p []EventPolicy, name string) map[string]interface{} {
		for _, entry := range p {
			if entry.EventType == name {
				return entry.Policy
			}
		}
		return nil
	}

	for name, tc := range map[string]struct {
		event  string
		cfg    map[string]interface{}
		policy map[string]interface{}
	}{
		"assign different default policy": {
			"error",
			map[string]interface{}{
				"mapping": []map[string]interface{}{{"event_type": "error", "policy_name": rollover30Days}}},
			policyPool()[rollover30Days]},
		"change default policy": {
			"transaction",
			map[string]interface{}{
				"policies": []map[string]interface{}{{
					"name":   rollover30Days,
					"policy": map[string]interface{}{"phases": map[string]interface{}{"delete": nil}}}}},
			map[string]interface{}{"policy": map[string]interface{}{"phases": map[string]interface{}{"delete": map[string]interface{}{}}}}},
		"assign new policy": {
			"span",
			map[string]interface{}{
				"mapping": []map[string]interface{}{{"event_type": "span", "policy_name": "delete-7-days"}},
				"policies": []map[string]interface{}{{
					"name":   "delete-7-days",
					"policy": map[string]interface{}{"phases": map[string]interface{}{"delete": nil}}}}},
			map[string]interface{}{"policy": map[string]interface{}{"phases": map[string]interface{}{"delete": map[string]interface{}{}}}}},
		"reference missing policy": {
			"span",
			map[string]interface{}{
				"require_policy": false,
				"mapping":        []map[string]interface{}{{"event_type": "span", "policy_name": "foo"}}},
			nil},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewConfig(common.MustNewConfigFrom(map[string]interface{}{"setup": tc.cfg}))
			require.NoError(t, err)
			assert.Equal(t, tc.policy, findPolicy(c.Setup.Policies, tc.event))
		})

		t.Run("unmanaged "+name, func(t *testing.T) {
			tc.cfg["enabled"] = false
			c, err := NewConfig(common.MustNewConfigFrom(map[string]interface{}{"setup": tc.cfg}))
			require.NoError(t, err)
			assert.ObjectsAreEqual(defaultPolicies(), c.Setup.Policies)

		})
	}
}

func TestConfig_Invalid(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg    map[string]interface{}
		errMsg string
	}{
		"invalid event_type": {map[string]interface{}{"mapping": []map[string]interface{}{{"event_type": "xyz", "policy_name": rollover30Days}}}, "event_type 'xyz' not supported"},
		"invalid policy":     {map[string]interface{}{"mapping": []map[string]interface{}{{"event_type": "span", "policy_name": "xyz"}}}, "policy 'xyz' not configured"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewConfig(common.MustNewConfigFrom(map[string]interface{}{"setup": tc.cfg}))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}

}

func defaultPolicies() []EventPolicy {
	var policies []EventPolicy
	for event, policyName := range policyMapping() {
		policies = append(policies, EventPolicy{EventType: event, Policy: policyPool()[policyName], Name: policyName})
	}
	return policies
}
