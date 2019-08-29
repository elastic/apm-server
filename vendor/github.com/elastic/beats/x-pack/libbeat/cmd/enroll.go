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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/beats/libbeat/cmd/instance"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/cli"
	"github.com/elastic/beats/x-pack/libbeat/management"
	"github.com/elastic/beats/x-pack/libbeat/management/api"
)

func getBeat(name, version string) (*instance.Beat, error) {
	b, err := instance.NewInitializedBeat(instance.Settings{Name: name, Version: version})
	if err != nil {
		return nil, fmt.Errorf("error creating beat: %s", err)
	}
	return b, nil
}

func genEnrollCmd(name, version string) *cobra.Command {
	var username, password string
	var force bool

	enrollCmd := cobra.Command{
		Use:   "enroll <kibana_url> [<enrollment_token>]",
		Short: "Enroll in Kibana for Central Management",
		Long: `This will enroll in  Kibana Beats Central Management. If you pass an enrollment token
		it will be used. You can also enroll using a username and password combination.`,
		Args: cobra.RangeArgs(1, 2),
		Run: cli.RunWith(
			func(cmd *cobra.Command, args []string) error {
				beat, err := getBeat(name, version)
				if err != nil {
					return err
				}

				kibanaURL := args[0]

				if username == "" && len(args) == 1 {
					return errors.New("You should pass either an enrollment token or use --username flag")
				}

				// Retrieve any available configuration avaible for Kibana, either
				// from the configuration file or using `-E`.
				kibanaRaw, err := kibanaConfig(beat.Config.Management)
				if err != nil {
					return err
				}

				// retrieve an enrollment token using username/password
				config, err := api.ConfigFromURL(kibanaURL, kibanaRaw)
				if err != nil {
					return err
				}

				confirm, err := confirmConfigOverwrite(force)
				if err != nil {
					return err
				}

				if !confirm {
					fmt.Println("Enrollment was canceled by the user")
					return nil
				}

				var enrollmentToken string
				if len(args) == 2 {
					// use given enrollment token
					enrollmentToken = args[1]
				} else {
					// pass username/password
					config.IgnoreVersion = true
					config.Username = username
					config.Password, err = cli.ReadPassword(password)
					if err != nil {
						return err
					}

					client, err := api.NewClient(config)
					if err != nil {
						return err
					}
					enrollmentToken, err = client.CreateEnrollmentToken()
					if err != nil {
						return errors.Wrap(err, "Error creating a new enrollment token")
					}
				}

				err = management.Enroll(beat, config, enrollmentToken)
				if err != nil {
					return errors.Wrap(err, "Error while enrolling")
				}

				fmt.Println("Enrolled and ready to retrieve settings from Kibana")
				return nil
			}),
	}

	enrollCmd.Flags().StringVar(&username, "username", "elastic", "Username to use when enrolling without token")
	enrollCmd.Flags().StringVar(&password, "password", "stdin", "Method to read the password to use when enrolling without token (stdin or env:VAR_NAME)")
	enrollCmd.Flags().BoolVar(&force, "force", false, "Force overwrite of current configuraiton, do not prompt for confirmation")

	return &enrollCmd
}

func kibanaConfig(config *common.Config) (*common.Config, error) {
	if config != nil && config.HasField("kibana") {
		sub, err := config.Child("kibana", -1)
		if err != nil {
			return nil, err
		}
		return sub, nil
	}
	return common.NewConfig(), nil
}

func confirmConfigOverwrite(force bool) (bool, error) {
	if force {
		return true, nil
	}

	return cli.Confirm("This will replace your current settings. Do you want to continue?", true)
}
