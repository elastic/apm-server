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
	"context"
	"flag"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/elastic/elastic-agent-libs/service"
)

func genRunCmd(params BeatParams) *cobra.Command {
	runCommand := cobra.Command{
		Use:   "run",
		Short: "Run APM Server",
		RunE: func(cmd *cobra.Command, args []string) error {
			beat, err := NewBeat(params)
			if err != nil {
				return err
			}
			// NOTE(axw) service.HandleSignals and service.NotifyTermination
			// may only be called once for a process's lifetime, so we handle
			// Windows service events here rather than in Run to enable testing
			// of Run.
			ctx, cancel := context.WithCancel(context.Background())
			service.HandleSignals(cancel, cancel)
			defer service.NotifyTermination()
			defer cancel()
			return beat.Run(ctx)
		},
	}

	// runGlobalFlags is a list of flags globally registered by various
	// libbeat packages, relevant only to the "run" command - add them
	// as subcommand flags.
	runGlobalFlags := []string{"N", "httpprof", "cpuprofile", "memprofile"}
	for _, flagName := range runGlobalFlags {
		runCommand.Flags().AddGoFlag(flag.CommandLine.Lookup(flagName))
	}

	runCommand.Flags().AddFlagSet(pflag.NewFlagSet("apm-server", pflag.ExitOnError))
	return &runCommand
}
