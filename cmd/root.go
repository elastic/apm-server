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

package cmd

import (
	"errors"
	"os"

	"github.com/spf13/pflag"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/cfgfile"
	"github.com/elastic/beats/v7/libbeat/cmd"
	"github.com/elastic/beats/v7/libbeat/cmd/instance"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring/report"
	"github.com/elastic/beats/v7/libbeat/publisher/processing"

	"github.com/elastic/apm-server/idxmgmt"
)

const (
	beatName = "apm-server"
	cloudEnv = "CLOUD_APM_CAPACITY"
)

var libbeatConfigOverrides = func() []cfgfile.ConditionalOverride {
	return []cfgfile.ConditionalOverride{
		{
			Check: func(_ *common.Config) bool {
				return true
			},
			Config: common.MustNewConfigFrom(map[string]interface{}{
				"logging": map[string]interface{}{
					"metrics": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
		},
		{
			Check: func(_ *common.Config) bool {
				return os.Getenv(cloudEnv) != ""
			},
			Config: func() *common.Config {
				return common.MustNewConfigFrom(map[string]interface{}{
					// default to medium compression on cloud
					"output.elasticsearch.compression_level": 5,
				})
			}(),
		}}
}

// DefaultSettings return the default settings for APM Server to pass into
// the GenRootCmdWithSettings.
func DefaultSettings() instance.Settings {
	return instance.Settings{
		Name:     beatName,
		Version:  defaultBeatVersion,
		RunFlags: pflag.NewFlagSet(beatName, pflag.ExitOnError),
		Monitoring: report.Settings{
			DefaultUsername: "apm_system",
		},
		IndexManagement: idxmgmt.NewSupporter,
		Processing:      processingSupport,
		ConfigOverrides: libbeatConfigOverrides(),
	}
}

func processingSupport(_ beat.Info, _ *logp.Logger, beatCfg *common.Config) (processing.Supporter, error) {
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
