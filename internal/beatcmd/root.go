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
	"errors"

	"github.com/spf13/pflag"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/cfgfile"
	"github.com/elastic/beats/v7/libbeat/cmd"
	"github.com/elastic/beats/v7/libbeat/cmd/instance"
	"github.com/elastic/beats/v7/libbeat/monitoring/report"
	"github.com/elastic/beats/v7/libbeat/publisher/processing"
	agentconfig "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/internal/idxmgmt"
	"github.com/elastic/apm-server/internal/version"
)

const (
	beatName = "apm-server"
)

var libbeatConfigOverrides = func() []cfgfile.ConditionalOverride {
	return []cfgfile.ConditionalOverride{
		{
			Check: func(_ *agentconfig.C) bool {
				return true
			},
			Config: agentconfig.MustNewConfigFrom(map[string]interface{}{
				"logging": map[string]interface{}{
					"metrics": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
		},
		{
			Check: func(_ *agentconfig.C) bool {
				return true
			},
			Config: func() *agentconfig.C {
				// default to turning off seccomp
				return agentconfig.MustNewConfigFrom(map[string]interface{}{
					"seccomp.enabled": false,
				})
			}(),
		},
	}
}

// DefaultSettings return the default settings for APM Server to pass into
// the GenRootCmdWithSettings.
func DefaultSettings() instance.Settings {
	return instance.Settings{
		Name:     beatName,
		Version:  version.Version,
		RunFlags: pflag.NewFlagSet(beatName, pflag.ExitOnError),
		Monitoring: report.Settings{
			DefaultUsername: "apm_system",
		},
		IndexManagement: idxmgmt.NewSupporter,
		Processing:      processingSupport,
		ConfigOverrides: libbeatConfigOverrides(),
	}
}

func processingSupport(_ beat.Info, _ *logp.Logger, beatCfg *agentconfig.C) (processing.Supporter, error) {
	if beatCfg.HasField("processors") {
		return nil, errors.New("libbeat processors are not supported")
	}
	return processingSupporter{}, nil
}

type processingSupporter struct{}

func (processingSupporter) Close() error {
	return nil
}

func (processingSupporter) Create(cfg beat.ProcessingConfig, _ bool) (beat.Processor, error) {
	return cfg.Processor, nil
}

// NewRootCommand returns the "apm-server" root command.
func NewRootCommand(newBeat beat.Creator, settings instance.Settings) *cmd.BeatsRootCmd {
	rootCmd := cmd.GenRootCmdWithSettings(newBeat, settings)
	rootCmd.AddCommand(genApikeyCmd(settings))
	modifyBuiltinCommands(rootCmd, settings)
	return rootCmd
}

func modifyBuiltinCommands(rootCmd *cmd.BeatsRootCmd, settings instance.Settings) {
	for _, cmd := range rootCmd.ExportCmd.Commands() {
		switch cmd.Name() {
		case "dashboard", "ilm-policy", "index-pattern", "template":
			// Remove unsupported "export" subcommands.
			rootCmd.ExportCmd.RemoveCommand(cmd)
		}
	}
	rootCmd.RemoveCommand(rootCmd.SetupCmd)
}
