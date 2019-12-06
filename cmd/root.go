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

	"github.com/elastic/beats/libbeat/cfgfile"

	"github.com/spf13/pflag"

	"github.com/elastic/apm-server/beater"
	"github.com/elastic/apm-server/idxmgmt"
	_ "github.com/elastic/apm-server/include"
	"github.com/elastic/beats/libbeat/cmd"
	"github.com/elastic/beats/libbeat/cmd/instance"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring/report"
	"github.com/elastic/beats/libbeat/publisher/processing"
)

// Name of the beat (apm-server).
const Name = "apm-server"

// IdxPattern for apm
const IdxPattern = "apm"

// RootCmd for running apm-server.
// This is the command that is used if no other command is specified.
// Running `apm-server run` or `apm-server` is identical.
var RootCmd *cmd.BeatsRootCmd

func init() {
	overrides := common.MustNewConfigFrom(map[string]interface{}{
		"logging": map[string]interface{}{
			"metrics": map[string]interface{}{
				"enabled": false,
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

	var runFlags = pflag.NewFlagSet(Name, pflag.ExitOnError)
	settings := instance.Settings{
		Name:        Name,
		IndexPrefix: IdxPattern,
		Version:     "",
		RunFlags:    runFlags,
		Monitoring: report.Settings{
			DefaultUsername: "apm_system",
		},
		IndexManagement: idxmgmt.MakeDefaultSupporter,
		Processing:      processing.MakeDefaultObserverSupport(false),
		ConfigOverrides: []cfgfile.ConditionalOverride{
			{
				Check: func(_ *common.Config) bool {
					return true
				},
				Config: overrides,
			},
		},
	}
	RootCmd = cmd.GenRootCmdWithSettings(beater.New, settings)

	for _, cmd := range RootCmd.ExportCmd.Commands() {

		// remove `dashboard` from `export` commands
		if cmd.Name() == "dashboard" {
			RootCmd.ExportCmd.RemoveCommand(cmd)
			continue
		}

		// only add defined flags to `export template` command
		if cmd.Name() == "template" {
			cmd.ResetFlags()
			cmd.Flags().String("es.version", settings.Version, "Elasticsearch version")
			cmd.Flags().String("dir", "", "Specify directory for printing template files. By default templates are printed to stdout.")
		}
	}
	// only add defined flags to setup command
	setup := RootCmd.SetupCmd
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
