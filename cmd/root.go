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

	"github.com/spf13/cobra"

	auth "github.com/elastic/apm-server/beater/authorization"

	"github.com/elastic/apm-server/beater/config"
	es "github.com/elastic/apm-server/elasticsearch"

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
type ApmCmd struct {
	*cmd.BeatsRootCmd
}

var RootCmd = ApmCmd{}

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
	RootCmd = ApmCmd{cmd.GenRootCmdWithSettings(beater.New, settings)}
	RootCmd.AddCommand(genApikeyCmd(settings))
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

func genApikeyCmd(settings instance.Settings) *cobra.Command {

	var client es.Client
	apmConfig, err := bootstrap(settings)
	if err == nil {
		client, err = es.NewClient(apmConfig.APIKeyConfig.ESConfig)
	}

	short := "Manage API Keys for communication between APM agents and server"
	apikeyCmd := cobra.Command{
		Use:   "apikey",
		Short: short,
		Long: short + `. 
Most operations require the "manage_security" cluster privilege. Ensure to configure "apm-server.api_key.*" or 
"output.elasticsearch.*" appropriately. APM Server will create security privileges for the "apm" application; 
you can freely query them. If you modify or delete apm privileges, APM Server might reject all requests.
If an invalid argument is passed, nothing will be printed.
Check the Elastic Security API documentation for details.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return err
		},
	}

	apikeyCmd.AddCommand(
		createApikeyCmd(client),
		invalidateApikeyCmd(client),
		getApikeysCmd(client),
		verifyApikeyCmd(apmConfig),
	)

	return &apikeyCmd
}

func createApikeyCmd(client es.Client) *cobra.Command {
	var keyName, expiration string
	var ingest, sourcemap, agentConfig, json bool
	short := "Create an API Key with the specified privilege(s)"
	create := &cobra.Command{
		Use:   "create",
		Short: short,
		Long: short + `.
If no privilege(s) are specified, the API Key will be valid for all.
Requires the "manage_security" cluster privilege in Elasticsearch.`,
		// always need to return error for possible scripts checking the exit code,
		// but printing the error must be done inside
		RunE: func(cmd *cobra.Command, args []string) error {
			privileges := booleansToPrivileges(ingest, sourcemap, agentConfig)
			return createApiKeyWithPrivileges(client, keyName, expiration, privileges, json)
		},
		// these are needed to not break JSON formatting
		// this has the caveat that if an invalid argument is passed, the command won't return anything
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	create.Flags().StringVar(&keyName, "name", "apm-key", "API Key name")
	create.Flags().StringVar(&expiration, "expiration", "",
		`expiration for the key, eg. "1d" (default never)`)
	create.Flags().BoolVar(&ingest, "ingest", false,
		fmt.Sprintf("give the %v privilege to this key, required for ingesting events", auth.PrivilegeEventWrite))
	create.Flags().BoolVar(&sourcemap, "sourcemap", false,
		fmt.Sprintf("give the %v privilege to this key, required for uploading sourcemaps",
			auth.PrivilegeSourcemapWrite))
	create.Flags().BoolVar(&agentConfig, "agent-config", false,
		fmt.Sprintf("give the %v privilege to this key, required for agents to read configuration remotely",
			auth.PrivilegeAgentConfigRead))
	create.Flags().BoolVar(&json, "json", false,
		"prints the output of this command as JSON")
	// this actually means "preserve sorting given in code" and not reorder them alphabetically
	create.Flags().SortFlags = false
	return create
}

func invalidateApikeyCmd(client es.Client) *cobra.Command {
	var id, name string
	var purge, json bool
	short := "Invalidate API Key(s) by Id or Name"
	invalidate := &cobra.Command{
		Use:   "invalidate",
		Short: short,
		Long: short + `.
If both "id" and "name" are supplied, only "id" will be used.
If neither of them are, an error will be returned.
Requires the "manage_security" cluster privilege in Elasticsearch.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return invalidateApiKey(client, &id, &name, purge, json)
		},
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	invalidate.Flags().StringVar(&id, "id", "", "id of the API Key to delete")
	invalidate.Flags().StringVar(&name, "name", "",
		"name of the API Key(s) to delete (several might match)")
	invalidate.Flags().BoolVar(&purge, "purge", false,
		"also remove all privileges created and used by APM Server")
	invalidate.Flags().BoolVar(&json, "json", false,
		"prints the output of this command as JSON")
	invalidate.Flags().SortFlags = false
	return invalidate
}

func getApikeysCmd(client es.Client) *cobra.Command {
	var id, name string
	var validOnly, json bool
	short := "Query API Key(s) by Id or Name"
	info := &cobra.Command{
		Use:   "info",
		Short: short,
		Long: short + `.
If both "id" and "name" are supplied, only "id" will be used.
If neither of them are, an error will be returned.
Requires the "manage_security" cluster privilege in Elasticsearch.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getApiKey(client, &id, &name, validOnly, json)
		},
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	info.Flags().StringVar(&id, "id", "", "id of the API Key to query")
	info.Flags().StringVar(&name, "name", "",
		"name of the API Key(s) to query (several might match)")
	info.Flags().BoolVar(&validOnly, "valid-only", false,
		"only return valid API Keys (not expired or invalidated)")
	info.Flags().BoolVar(&json, "json", false,
		"prints the output of this command as JSON")
	info.Flags().SortFlags = false
	return info
}

func verifyApikeyCmd(config *config.Config) *cobra.Command {
	var credentials string
	var ingest, sourcemap, agentConfig, json bool
	short := `Check if a "credentials" string has the given privilege(s)`
	long := short + `.
If no privilege(s) are specified, the credentials will be queried for all.`
	verify := &cobra.Command{
		Use:   "verify",
		Short: short,
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			privileges := booleansToPrivileges(ingest, sourcemap, agentConfig)
			return verifyApiKey(config, privileges, credentials, json)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	verify.Flags().StringVar(&credentials, "credentials", "", `credentials for which check privileges`)
	verify.Flags().BoolVar(&ingest, "ingest", false,
		fmt.Sprintf("ask for the %v privilege, required for ingesting events", auth.PrivilegeEventWrite))
	verify.Flags().BoolVar(&sourcemap, "sourcemap", false,
		fmt.Sprintf("ask for the %v privilege, required for uploading sourcemaps",
			auth.PrivilegeSourcemapWrite))
	verify.Flags().BoolVar(&agentConfig, "agent-config", false,
		fmt.Sprintf("ask for the %v privilege, required for agents to read configuration remotely",
			auth.PrivilegeAgentConfigRead))
	verify.Flags().BoolVar(&json, "json", false,
		"prints the output of this command as JSON")
	verify.Flags().SortFlags = false

	return verify
}

// created the beat, instantiate configuration, and so on
// apm-server.api_key.enabled is implicitly true
func bootstrap(settings instance.Settings) (*config.Config, error) {
	beat, err := instance.NewBeat(settings.Name, settings.IndexPrefix, settings.Version)
	if err != nil {
		return nil, err
	}
	err = beat.InitWithSettings(settings)
	if err != nil {
		return nil, err
	}

	outCfg := beat.Config.Output
	return config.NewConfig(settings.Version, beat.RawConfig, outCfg.Config())
}

// if all are false, returns any ("*")
// this is because Elasticsearch requires at least 1 privilege for most queries,
// so "*" acts as default
func booleansToPrivileges(ingest, sourcemap, agentConfig bool) []es.Privilege {
	privileges := make([]es.Privilege, 0)
	if ingest {
		privileges = append(privileges, auth.PrivilegeEventWrite.Action)
	}
	if sourcemap {
		privileges = append(privileges, auth.PrivilegeSourcemapWrite.Action)
	}
	if agentConfig {
		privileges = append(privileges, auth.PrivilegeAgentConfigRead.Action)
	}
	any := ingest || sourcemap || agentConfig
	if !any {
		privileges = append(privileges, auth.ActionAny)
	}
	return privileges
}
