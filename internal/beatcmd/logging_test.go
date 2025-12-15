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

package beatcmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
)

var environments = []logp.Environment{
	logp.DefaultEnvironment,
	logp.SystemdEnvironment,
	logp.ContainerEnvironment,
	logp.MacOSServiceEnvironment,
	logp.WindowsServiceEnvironment,
	logp.InvalidEnvironment,
}

func TestBuildLoggingConfigPerEnvironment(t *testing.T) {
	for _, env := range environments {
		for _, tt := range []struct {
			name           string
			cfg            *Config
			logStderr      bool
			debugSelectors []string
			opts           []logp.Option

			expectedErr         error
			buildExpectedConfig func() logp.Config
		}{
			{
				name: fmt.Sprintf("with an empty config and %s environment", env),
				cfg:  &Config{},

				buildExpectedConfig: func() logp.Config {
					cfg := logp.DefaultConfig(env)
					cfg.Beat = "apm-server"
					return cfg
				},
			},
			{
				name: fmt.Sprintf("with an logging config specified and %s environment", env),
				cfg: &Config{
					Logging: config.NewConfig(),
				},

				buildExpectedConfig: func() logp.Config {
					cfg := logp.DefaultConfig(env)
					cfg.Beat = "apm-server"
					return cfg
				},
			},
			{
				name:      fmt.Sprintf("when logging to stderr and %s environment", env),
				cfg:       &Config{},
				logStderr: true,

				buildExpectedConfig: func() logp.Config {
					cfg := logp.DefaultConfig(env)
					cfg.Beat = "apm-server"
					cfg.ToStderr = true
					return cfg
				},
			},
			{
				name:           fmt.Sprintf("with debug selectors and %s environment", env),
				cfg:            &Config{},
				debugSelectors: []string{"hello,world", "bonjour"},

				buildExpectedConfig: func() logp.Config {
					cfg := logp.DefaultConfig(env)
					cfg.Beat = "apm-server"
					cfg.Level = -1
					cfg.Selectors = []string{"hello", "world", "bonjour"}
					return cfg
				},
			},
			{
				name: fmt.Sprintf("with options and %s environment", env),
				cfg:  &Config{},
				opts: []logp.Option{
					logp.WithLevel(logp.DebugLevel),
				},

				buildExpectedConfig: func() logp.Config {
					cfg := logp.DefaultConfig(env)
					cfg.Beat = "apm-server"
					cfg.Level = -1
					return cfg
				},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				cfg, err := buildLoggingConfig(tt.cfg, env, tt.logStderr, tt.debugSelectors, tt.opts...)

				if tt.expectedErr == nil {
					assert.NoError(t, err)
				} else {
					assert.Equal(t, tt.expectedErr, err)
				}

				assert.Equal(t, tt.buildExpectedConfig(), cfg)
			})
		}
	}
}
