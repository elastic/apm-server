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

package main

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/elastic/apm-server/systemtest/fleettest"
)

var (
	kibanaURL string
)

func parseVars(s ...string) (map[string]string, error) {
	out := make(map[string]string)
	for _, s := range s {
		i := strings.IndexRune(s, '=')
		if i < 0 {
			return nil, errors.New("missing '='; expected format k=v")
		}
		k, vstr := s[:i], s[i+1:]
		out[k] = vstr
	}
	return out, nil
}

func command() *cobra.Command {
	listPackagePoliciesCommand := &cobra.Command{
		Use:   "list-policies",
		Short: "List Fleet integration package policies",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := fleettest.NewClient(kibanaURL)
			policies, err := client.ListPackagePolicies()
			if err != nil {
				return fmt.Errorf("failed to fetch package policies: %w", err)
			}
			return yaml.NewEncoder(cmd.OutOrStdout()).Encode(policies)
		},
	}

	updatePackagePolicyCommand := &cobra.Command{
		Use:   "set-policy-var <policy-id> <k=v [k=v...]>",
		Short: "Set config vars for a Fleet integration package policy",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			setVars, err := parseVars(args[1:]...)
			if err != nil {
				return err
			}

			client := fleettest.NewClient(kibanaURL)
			policy, err := client.PackagePolicy(args[0])
			if err != nil {
				return fmt.Errorf("failed to fetch package policy: %w", err)
			}
			if len(policy.Inputs) != 1 {
				return fmt.Errorf("expected 1 input, got %d", len(policy.Inputs))
			}

			for k, v := range setVars {
				varObj, ok := policy.Inputs[0].Vars[k].(map[string]interface{})
				if !ok {
					return fmt.Errorf("var %q not found in package policy", k)
				}
				value := varObj["value"]
				ptrZero := reflect.New(reflect.TypeOf(value))
				if err := yaml.Unmarshal([]byte(v), ptrZero.Interface()); err != nil {
					return fmt.Errorf("failed to unmarshal var %q: %w", k, err)
				}
				varObj["value"] = ptrZero.Elem().Interface()
			}

			if err := client.UpdatePackagePolicy(policy); err != nil {
				return fmt.Errorf("failed to update policy: %w", err)
			}
			return nil
		},
	}

	rootCommand := &cobra.Command{Use: "fleetctl"}
	rootCommand.AddCommand(listPackagePoliciesCommand)
	rootCommand.AddCommand(updatePackagePolicyCommand)
	rootCommand.PersistentFlags().StringVarP(&kibanaURL, "kibana", "u", "http://localhost:5601", "URL of the Kibana server")
	return rootCommand
}

func main() {
	if err := command().Execute(); err != nil {
		log.Fatal(err)
	}
}
