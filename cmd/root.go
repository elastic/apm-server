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
)

var libbeatConfigOverrides = common.MustNewConfigFrom(map[string]interface{}{
	"logging": map[string]interface{}{
		"metrics": map[string]interface{}{
			"enabled": false,
		},
		"files": map[string]interface{}{
			"rotateeverybytes": 10 * 1024 * 1024,
		},
	},
	"setup": map[string]interface{}{
		"template": map[string]interface{}{
			"settings": map[string]interface{}{
				"index": map[string]interface{}{
					"codec": "best_compression",
					"mapping": map[string]interface{}{
						"total_fields": map[string]int{
							"limit": 2000,
						},
					},
					"number_of_shards": 1,
				},
				"_source": map[string]interface{}{
					"enabled": true,
				},
			},
		},
	},
})

// NewRootCommand returns the "apm-server" root command.
func NewRootCommand(newBeat beat.Creator) *cmd.BeatsRootCmd {

	var runFlags = pflag.NewFlagSet(beatName, pflag.ExitOnError)
	settings := instance.Settings{
		Name:        beatName,
		IndexPrefix: apmIndexPattern,
		Version:     "",
		RunFlags:    runFlags,
		Monitoring: report.Settings{
			DefaultUsername: "apm_system",
		},
		IndexManagement: idxmgmt.MakeDefaultSupporter,
		Processing:      processing.MakeDefaultObserverSupport(false),
		ConfigOverrides: []cfgfile.ConditionalOverride{{
			Check: func(_ *common.Config) bool {
				return true
			},
			Config: libbeatConfigOverrides,
		}},
	}

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
