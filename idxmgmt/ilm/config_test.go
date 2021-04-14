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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	libilm "github.com/elastic/beats/v7/libbeat/idxmgmt/ilm"
)

var mockBeatInfo = beat.Info{Beat: "mockapm", Version: "9.9.9"}

func TestConfig_Default(t *testing.T) {
	c, err := NewConfig(mockBeatInfo, nil)
	require.NoError(t, err)
	expectedCfg := Config{
		Mode: libilm.ModeAuto,
		Setup: Setup{
			Enabled:       true,
			Overwrite:     false,
			RequirePolicy: true,
			Mappings:      defaultMappingsResolved(mockBeatInfo),
			Policies:      defaultPolicies()}}
	assert.Equal(t, expectedCfg, c)
}

func TestConfig_Mode(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg      string
		expected libilm.Mode
	}{
		"default":  {`{"enabled":"auto"}`, libilm.ModeAuto},
		"disabled": {`{"enabled":"false"}`, libilm.ModeDisabled},
		"enabled":  {`{"enabled":"true"}`, libilm.ModeEnabled},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewConfig(mockBeatInfo, common.MustNewConfigFrom(tc.cfg))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, c.Mode)
		})
	}
}

func TestConfig_SetupEnabled(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg     string
		enabled bool
	}{
		"enabled":  {`{"setup":{"enabled":true}}`, true},
		"disabled": {`{"setup":{"enabled":false}}`, false},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewConfig(mockBeatInfo, common.MustNewConfigFrom(tc.cfg))
			require.NoError(t, err)
			assert.Equal(t, tc.enabled, c.Setup.Enabled)
		})
	}
}

func TestConfig_SetupOverwrite(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg       string
		overwrite bool
	}{
		"overwrite":        {`{"setup":{"overwrite":true}}`, true},
		"do not overwrite": {`{"setup":{"overwrite":false}}`, false},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewConfig(mockBeatInfo, common.MustNewConfigFrom(tc.cfg))
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
			c, err := NewConfig(mockBeatInfo, common.MustNewConfigFrom(map[string]interface{}{"setup": tc.cfg}))
			require.NoError(t, err)
			assert.Equal(t, tc.required, c.Setup.RequirePolicy)
		})
	}
}

func TestConfig_Valid(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name string
		cfg  string

		expected Config
	}{
		{name: "new policy and index suffix",
			cfg: `{"setup":{"mapping":[{"event_type":"span","policy_name":"spanPolicy"},{"event_type":"metric","index_suffix":"ProdUCtion"},{"event_type":"error","index_suffix":"%{[observer.name]}-%{+yyyy-MM-dd}"}],"policies":[{"name":"spanPolicy","policy":{"phases":{"foo":{}}}}]}}`,
			expected: Config{Mode: libilm.ModeAuto,
				Setup: Setup{Enabled: true, Overwrite: false, RequirePolicy: true,
					Mappings: map[string]Mapping{
						"error": {EventType: "error", PolicyName: defaultPolicyName,
							Index: fmt.Sprintf("apm-9.9.9-error-mockapm-%d-%02d-%02d", now.Year(), now.Month(), now.Day()), IndexSuffix: "%{[observer.name]}-%{+yyyy-MM-dd}"},
						"span": {EventType: "span", PolicyName: "spanPolicy",
							Index: "apm-9.9.9-span"},
						"transaction": {EventType: "transaction", PolicyName: defaultPolicyName,
							Index: "apm-9.9.9-transaction"},
						"metric": {EventType: "metric", PolicyName: defaultPolicyName,
							Index: "apm-9.9.9-metric-production", IndexSuffix: "ProdUCtion"},
						"profile": {EventType: "profile", PolicyName: defaultPolicyName,
							Index: "apm-9.9.9-profile"},
					},
					Policies: map[string]Policy{
						defaultPolicyName: defaultPolicies()[defaultPolicyName],
						"spanPolicy": {Name: "spanPolicy", Body: map[string]interface{}{
							"policy": map[string]interface{}{"phases": map[string]interface{}{
								"foo": map[string]interface{}{}}}}},
					},
				}},
		},
		{name: "changed default policy",
			cfg: `{"setup":{"policies":[{"name":"apm-rollover-30-days","policy":{"phases":{"warm":{"min_age":"30d"}}}}]}}`,
			expected: Config{Mode: libilm.ModeAuto,
				Setup: Setup{Enabled: true, Overwrite: false, RequirePolicy: true,
					Mappings: defaultMappingsResolved(mockBeatInfo),
					Policies: map[string]Policy{
						defaultPolicyName: {Name: defaultPolicyName, Body: map[string]interface{}{
							"policy": map[string]interface{}{"phases": map[string]interface{}{
								"warm": map[string]interface{}{"min_age": "30d"}}}}},
					},
				}},
		},
		{name: "allow unknown policy",
			cfg: `{"setup":{"require_policy":false,"mapping":[{"event_type":"error","policy_name":"errorPolicy"}]}}`,
			expected: Config{Mode: libilm.ModeAuto,
				Setup: Setup{Enabled: true, Overwrite: false, RequirePolicy: false,
					Mappings: func() map[string]Mapping {
						m := defaultMappingsResolved(mockBeatInfo)
						m["error"] = Mapping{EventType: "error", PolicyName: "errorPolicy",
							Index: "apm-9.9.9-error"}
						return m
					}(),
					Policies: func() map[string]Policy {
						p := defaultPolicies()
						p["errorPolicy"] = Policy{Name: "errorPolicy"}
						return p
					}(),
				}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := NewConfig(mockBeatInfo, common.MustNewConfigFrom(tc.cfg))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, cfg)
		})
	}

}

func TestConfig_Invalid(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cfg    string
		errMsg string
	}{
		{name: "invalid event_type",
			cfg:    `{"setup":{"mapping":[{"event_type": "xyz", "policy_name": "rollover30Days"}]}}`,
			errMsg: "event_type 'xyz' not supported"},
		{name: "invalid policy",
			cfg:    `{"setup":{"mapping":[{"event_type":"span","policy_name":"xyz"}]}}`,
			errMsg: "policy 'xyz' not configured"},
		{name: "invalid index suffix",
			cfg:    `{"setup":{"mapping":[{"event_type":"span","index_suffix":"%{[foo.version]}"}]}}`,
			errMsg: "index suffix cannot be resolved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewConfig(mockBeatInfo, common.MustNewConfigFrom(tc.cfg))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func defaultMappingsResolved(info beat.Info) map[string]Mapping {
	m := defaultMappings()
	for k, v := range m {
		v.Index = fmt.Sprintf("apm-%s-%s", info.Version, k)
		m[k] = v
	}
	return m
}
