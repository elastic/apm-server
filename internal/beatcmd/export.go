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
	"io"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/elastic/beats/v7/libbeat/common/cli"
)

var exportCommand = &cobra.Command{
	Use:   "export",
	Short: "Export current config",
}

var exportConfigCommand = &cobra.Command{
	Use:   "config",
	Short: "Export current config to stdout",
	Run: cli.RunWith(func(cmd *cobra.Command, args []string) error {
		return exportConfig(cmd.OutOrStdout())
	}),
}

func init() {
	exportCommand.AddCommand(exportConfigCommand)
}

func exportConfig(w io.Writer) error {
	_, rawConfig, _, err := LoadConfig(WithDisableConfigResolution())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	var config map[string]interface{}
	if err := rawConfig.Unpack(&config); err != nil {
		return fmt.Errorf("failed to unpack config: %w", err)
	}
	if err := yaml.NewEncoder(w).Encode(config); err != nil {
		return fmt.Errorf("failed to marshal config as YAML: %w", err)
	}
	return nil
}
