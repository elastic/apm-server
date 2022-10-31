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

	"github.com/spf13/cobra"

	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/testing"

	beaterconfig "github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/idxmgmt"
)

func genTestCmd(beatParams BeatParams) *cobra.Command {
	exportCmd := &cobra.Command{
		Use:   "test",
		Short: "Test config",
	}
	exportCmd.AddCommand(testConfigCommand)
	exportCmd.AddCommand(newTestOutputCommand(beatParams))
	return exportCmd
}

var testConfigCommand = &cobra.Command{
	Use:   "config",
	Short: "Test configuration settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, _, err := LoadConfig()
		if err != nil {
			return err
		}

		// Validate the APM Server config.
		var esOutputConfig *config.C
		if cfg.Output.Name() == "elasticsearch" {
			esOutputConfig = cfg.Output.Config()
		}
		if _, err := beaterconfig.NewConfig(cfg.APMServer, esOutputConfig); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Config OK")
		return nil
	},
}

func newTestOutputCommand(beatParams BeatParams) *cobra.Command {
	return &cobra.Command{
		Use:   "output",
		Short: "Test that APM Server can connect to the output by using the current settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			beat, err := NewBeat(beatParams)
			if err != nil {
				return err
			}
			indexSupporter := idxmgmt.NewSupporter(nil, beat.rawConfig)
			output, err := outputs.Load(
				indexSupporter, beat.Info, nil, beat.Config.Output.Name(), beat.Config.Output.Config(),
			)
			if err != nil {
				return fmt.Errorf("error initializing output: %w", err)
			}
			for _, client := range output.Clients {
				client, ok := client.(testing.Testable)
				if !ok {
					return fmt.Errorf("%s output doesn't support testing", beat.Config.Output.Name())
				}
				client.Test(testing.NewConsoleDriver(cmd.OutOrStdout()))
			}
			return nil
		},
	}
}
