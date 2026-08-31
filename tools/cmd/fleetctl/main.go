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
	"net/url"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/elastic/apm-server/systemtest/fleettest"
)

var client *fleettest.Client

func parseKV(s ...string) (map[string]string, error) {
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

var listPackagePoliciesCommand = &cobra.Command{
	Use:   "list-policies",
	Short: "List Fleet integration package policies",
	RunE: func(cmd *cobra.Command, _ []string) error {
		policies, err := client.ListPackagePolicies()
		if err != nil {
			return fmt.Errorf("failed to fetch package policies: %w", err)
		}
		return yaml.NewEncoder(cmd.OutOrStdout()).Encode(policies)
	},
}

var setPolicyVarCommand = &cobra.Command{
	Use:   "set-policy-var <policy-id> <k=v [k=v...]>",
	Short: "Set config vars for a Fleet integration package policy",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		setVars, err := parseKV(args[1:]...)
		if err != nil {
			return err
		}

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

var setPolicyConfigCommand = &cobra.Command{
	Use:   "set-policy-config <policy-id> <k=v [k=v...]>",
	Short: "Set arbitrary config for a Fleet integration package policy",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		config, err := parseKV(args[1:]...)
		if err != nil {
			return err
		}

		policy, err := client.PackagePolicy(args[0])
		if err != nil {
			return fmt.Errorf("failed to fetch package policy: %w", err)
		}
		if len(policy.Inputs) != 1 {
			return fmt.Errorf("expected 1 input, got %d", len(policy.Inputs))
		}

		merge := func(k string, v interface{}, to map[string]interface{}) {
			for {
				before, after, found := strings.Cut(k, ".")
				if !found {
					to[before] = v
					return
				}
				m, ok := to[before].(map[string]interface{})
				if !ok {
					m = make(map[string]interface{})
					to[before] = m
				}
				k = after
				to = m
			}
		}

		existing := policy.Inputs[0].Config
		if existing == nil {
			existing = make(map[string]interface{})
			policy.Inputs[0].Config = existing
		}
		for k, v := range config {
			var value interface{}
			if err := yaml.Unmarshal([]byte(v), &value); err != nil {
				return fmt.Errorf("failed to unmarshal var %q: %w", k, err)
			}
			// Each top-level key's value is nested under "value".
			if before, after, ok := strings.Cut(k, "."); ok {
				k = strings.Join([]string{before, "value", after}, ".")
			} else {
				k = before + ".value"
			}
			merge(k, value, existing)
		}

		if err := client.UpdatePackagePolicy(policy); err != nil {
			return fmt.Errorf("failed to update policy: %w", err)
		}
		return nil
	},
}

func main() {
	var (
		kibanaURL  string
		kibanaUser string
		kibanaPass string
	)
	rootCommand := &cobra.Command{Use: "fleetctl",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			u, err := url.Parse(kibanaURL)
			if err != nil {
				return err
			}
			u.User = url.UserPassword(kibanaUser, kibanaPass)
			client = fleettest.NewClient(u.String())
			return nil
		},
	}
	rootCommand.AddCommand(listPackagePoliciesCommand)
	rootCommand.AddCommand(setPolicyVarCommand)
	rootCommand.AddCommand(setPolicyConfigCommand)
	rootCommand.PersistentFlags().StringVarP(&kibanaURL, "kibana", "u", "http://localhost:5601", "URL of the Kibana server")
	rootCommand.PersistentFlags().StringVar(&kibanaUser, "user", "admin", "Username to use for Kibana authentication")
	rootCommand.PersistentFlags().StringVar(&kibanaPass, "pass", "changeme", "Password to use for Kibana authentication")

	if err := rootCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}
