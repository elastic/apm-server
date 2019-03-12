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
	"github.com/spf13/pflag"

	"github.com/elastic/apm-server/beater"
	_ "github.com/elastic/apm-server/include"
	"github.com/elastic/beats/libbeat/cmd"
	"github.com/elastic/beats/libbeat/cmd/instance"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring/report"
)

// Name of the beat (apm-server).
const Name = "apm-server"
const IdxPattern = "apm"

// RootCmd for running apm-server.
// This is the command that is used if no other command is specified.
// Running `apm-server run` or `apm-server` is identical.
var RootCmd *cmd.BeatsRootCmd

func init() {
	overrides, _ := common.NewConfigFrom(map[string]interface{}{
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
				},
			},
		},
	})

	var runFlags = pflag.NewFlagSet(Name, pflag.ExitOnError)
	RootCmd = cmd.GenRootCmdWithSettings(beater.New, instance.Settings{
		ConfigOverrides: overrides,
		Name:            Name,
		IndexPrefix:     IdxPattern,
		Version:         "",
		RunFlags:        runFlags,
		Monitoring: report.Settings{
			DefaultUsername: "apm_system",
		},
	})

	// remove ilm-policy from export commands
	for _, cmd := range RootCmd.ExportCmd.Commands() {
		if cmd.Name() == "ilm-policy" {
			RootCmd.ExportCmd.RemoveCommand(cmd)
		}
	}
	// only add defined flags to setup command
	setup := RootCmd.SetupCmd
	setup.Short = "Setup Elasticsearch index template and pipelines"
	setup.Long = `This command does initial setup of the environment:
 * Index mapping template in Elasticsearch to ensure fields are mapped.
 * Ingest pipelines
 * Kibana dashboards
`
	setup.ResetFlags()
	setup.Flags().Bool("template", false, "Setup index template")
	// for compatibility reasons we need to still support dashboards.
	setup.Flags().Bool("dashboards", false, "Setup dashboards")
	setup.Flags().Bool("pipelines", false, "Setup Ingest pipelines")
}
