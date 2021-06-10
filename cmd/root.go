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
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/cfgfile"
	"github.com/elastic/beats/v7/libbeat/cmd"
	"github.com/elastic/beats/v7/libbeat/cmd/instance"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/monitoring/report"
	"github.com/elastic/beats/v7/libbeat/publisher/processing"

	"github.com/elastic/apm-server/idxmgmt"
	_ "github.com/elastic/apm-server/include" // include assets
)

const (
	beatName        = "apm-server"
	apmIndexPattern = "apm"
	cloudEnv        = "CLOUD_APM_CAPACITY"
)

type throughputSettings struct {
	worker      int
	bulkMaxSize int
	events      int
	minEvents   int
}

var cloudMatrix = map[string]throughputSettings{
	"512":  {5, 267, 2000, 267},
	"1024": {7, 381, 4000, 381},
	"2048": {10, 533, 8000, 533},
	"4096": {14, 762, 16000, 762},
	"8192": {20, 1067, 32000, 1067},
}

var libbeatConfigOverrides = func() []cfgfile.ConditionalOverride {
	return []cfgfile.ConditionalOverride{{
		Check: func(_ *common.Config) bool {
			return true
		},
		Config: common.MustNewConfigFrom(map[string]interface{}{
			"logging": map[string]interface{}{
				"metrics": map[string]interface{}{
					"enabled": false,
				},
				"ecs":  true,
				"json": true,
			},
		}),
	},
		{
			Check: func(_ *common.Config) bool {
				return true
			},
			Config: func() *common.Config {
				cap := os.Getenv(cloudEnv)
				if _, ok := cloudMatrix[cap]; !ok {
					return common.NewConfig()
				}
				return common.MustNewConfigFrom(map[string]interface{}{
					"output": map[string]interface{}{
						"elasticsearch": map[string]interface{}{
							"worker":        cloudMatrix[cap].worker,
							"bulk_max_size": cloudMatrix[cap].bulkMaxSize,
						},
					},
					"queue": map[string]interface{}{
						"mem": map[string]interface{}{
							"events": cloudMatrix[cap].events,
							"flush": map[string]interface{}{
								"min_events": cloudMatrix[cap].minEvents,
							},
						},
					},
				})
			}(),
		},
	}
}

// DefaultSettings return the default settings for APM Server to pass into
// the GenRootCmdWithSettings.
func DefaultSettings() instance.Settings {
	return instance.Settings{
		Name:        beatName,
		IndexPrefix: apmIndexPattern,
		Version:     defaultBeatVersion,
		RunFlags:    pflag.NewFlagSet(beatName, pflag.ExitOnError),
		Monitoring: report.Settings{
			DefaultUsername: "apm_system",
		},
		IndexManagement: idxmgmt.MakeDefaultSupporter,
		Processing:      processing.MakeDefaultObserverSupport(false),
		ConfigOverrides: libbeatConfigOverrides(),
	}
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
		case "dashboard":
			// remove `dashboard` from `export` commands
			rootCmd.ExportCmd.RemoveCommand(cmd)
		case "template":
			// only add defined flags to `export template` command
			cmd.ResetFlags()
			cmd.Flags().String("es.version", settings.Version, "Elasticsearch version")
			cmd.Flags().String("dir", "", "Specify directory for printing template files. By default templates are printed to stdout.")
		}
	}

	// only add defined flags to setup command
	setup := rootCmd.SetupCmd
	setup.Short = "Setup Elasticsearch index management components and pipelines"
	setup.Long = `This command does initial setup of the environment:

 * Index management including loading Elasticsearch templates, ILM policies and write aliases.
 * Ingest pipelines
`
	setup.ResetFlags()

	//lint:ignore SA1019 Setting up template must still be supported until next major version upgrade.
	tmplKey := cmd.TemplateKey
	setup.Flags().Bool(tmplKey, false, "Setup index template")
	setup.Flags().MarkDeprecated(tmplKey, fmt.Sprintf("please use --%s instead", cmd.IndexManagementKey))
	setup.Flags().Bool(cmd.IndexManagementKey, false, "Setup Elasticsearch index management")
	setup.Flags().Bool(cmd.PipelineKey, false, "Setup ingest pipelines")
}
