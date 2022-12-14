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
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/elastic/beats/v7/libbeat/cfgfile"
	"github.com/elastic/beats/v7/libbeat/cmd/platformcheck"
	"github.com/elastic/elastic-agent-libs/logp"
)

// NewRootCommand returns the root command for apm-server.
//
// NewRootCommand takes a BeatParams, which will be passed to
// commands that must create an instance of APM Server.
func NewRootCommand(beatParams BeatParams) *cobra.Command {
	if err := platformcheck.CheckNativePlatformCompat(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	err := cfgfile.ChangeDefaultCfgfileFlag("apm-server")
	if err != nil {
		panic(fmt.Errorf("failed to set default config file path: %v", err))
	}

	// root command is an alias for "run"
	runCommand := genRunCmd(beatParams)
	rootCommand := &cobra.Command{
		Use:  "apm-server",
		RunE: runCommand.RunE,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}
	rootCommand.Flags().AddFlagSet(runCommand.Flags())

	// globalFlags is a list of flags globally registered by various libbeat
	// packages, relevant to all commands - add them as persistent flags.
	globalFlags := []string{
		"E", "c",
		"path.config",
		"path.data",
		"path.logs",
		"path.home",
		"strict.perms",
	}
	for _, flagName := range globalFlags {
		rootCommand.PersistentFlags().AddGoFlag(flag.CommandLine.Lookup(flagName))
	}

	// Add logging-related flags to all commands.
	rootCommand.PersistentFlags().BoolVarP(&logVerbose, "v", "v", false, "Log at INFO level")
	rootCommand.PersistentFlags().BoolVarP(&logStderr, "e", "e", false, "Log to stderr and disable syslog/file output")
	rootCommand.PersistentFlags().StringArrayVarP(&logDebugSelectors, "d", "d", nil, "Enable certain debug selectors")
	rootCommand.PersistentFlags().Var(&logEnvironment, "environment", "Set the environment in which the process is running")

	// Register subcommands.
	rootCommand.AddCommand(runCommand)
	rootCommand.AddCommand(exportCommand)
	rootCommand.AddCommand(keystoreCommand)
	rootCommand.AddCommand(versionCommand)
	rootCommand.AddCommand(genTestCmd(beatParams))
	rootCommand.AddCommand(genApikeyCmd())

	return rootCommand
}

type logpEnvironmentVar struct {
	env logp.Environment
}

func (v *logpEnvironmentVar) Set(in string) error {
	env := logp.ParseEnvironment(in)
	if env == logp.InvalidEnvironment {
		return fmt.Errorf("invalid logging environment: %q", in)
	}
	v.env = env
	return nil
}

func (v *logpEnvironmentVar) Type() string {
	return "string"
}

func (v *logpEnvironmentVar) String() string {
	return v.env.String()
}
