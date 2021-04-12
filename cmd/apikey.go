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
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/elastic/beats/v7/libbeat/cfgfile"
	"github.com/elastic/beats/v7/libbeat/cmd/instance"
	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"

	auth "github.com/elastic/apm-server/beater/authorization"
	es "github.com/elastic/apm-server/elasticsearch"
)

func genApikeyCmd(settings instance.Settings) *cobra.Command {

	short := "Manage API Keys for communication between APM agents and server"
	apikeyCmd := cobra.Command{
		Use:   "apikey",
		Short: short,
		Long: short + `.
Most operations require the "manage_api_key" cluster privilege. Ensure to configure "apm-server.api_key.*" or
"output.elasticsearch.*" appropriately. APM Server will create security privileges for the "apm" application;
you can freely query them. If you modify or delete apm privileges, APM Server might reject all requests.
Check the Elastic Security API documentation for details.`,
	}

	apikeyCmd.AddCommand(
		createApikeyCmd(settings),
		invalidateApikeyCmd(settings),
		getApikeysCmd(settings),
		verifyApikeyCmd(settings),
	)
	return &apikeyCmd
}

func createApikeyCmd(settings instance.Settings) *cobra.Command {
	var keyName, expiration string
	var ingest, sourcemap, agentConfig, json bool
	short := "Create an API Key with the specified privilege(s)"
	create := &cobra.Command{
		Use:   "create",
		Short: short,
		Long: short + `.
If no privilege(s) are specified, the API Key will be valid for all.`,
		Run: makeAPIKeyRun(settings, &json, func(client es.Client, config *config.Config, args []string) error {
			privileges := booleansToPrivileges(ingest, sourcemap, agentConfig)
			if len(privileges) == 0 {
				// No privileges specified, grant all.
				privileges = auth.ActionsAll()
			}
			return createAPIKey(client, keyName, expiration, privileges, json)
		}),
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

func invalidateApikeyCmd(settings instance.Settings) *cobra.Command {
	var id, name string
	var json bool
	short := "Invalidate API Key(s) by Id or Name"
	invalidate := &cobra.Command{
		Use:   "invalidate",
		Short: short,
		Long: short + `.
If both "id" and "name" are supplied, only "id" will be used.
If neither of them are, an error will be returned.`,
		Run: makeAPIKeyRun(settings, &json, func(client es.Client, config *config.Config, args []string) error {
			if id == "" && name == "" {
				// TODO(axw) this should trigger usage
				return errors.New(`either "id" or "name" are required`)
			}
			return invalidateAPIKey(client, &id, &name, json)
		}),
	}
	invalidate.Flags().StringVar(&id, "id", "", "id of the API Key to delete")
	invalidate.Flags().StringVar(&name, "name", "",
		"name of the API Key(s) to delete (several might match)")
	invalidate.Flags().BoolVar(&json, "json", false,
		"prints the output of this command as JSON")
	invalidate.Flags().SortFlags = false
	return invalidate
}

func getApikeysCmd(settings instance.Settings) *cobra.Command {
	var id, name string
	var validOnly, json bool
	short := "Query API Key(s) by Id or Name"
	info := &cobra.Command{
		Use:   "info",
		Short: short,
		Long: short + `.
If both "id" and "name" are supplied, only "id" will be used.
If neither of them are, an error will be returned.`,
		Run: makeAPIKeyRun(settings, &json, func(client es.Client, config *config.Config, args []string) error {
			if id == "" && name == "" {
				// TODO(axw) this should trigger usage
				return errors.New(`either "id" or "name" are required`)
			}
			return getAPIKey(client, &id, &name, validOnly, json)
		}),
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

func verifyApikeyCmd(settings instance.Settings) *cobra.Command {
	var credentials string
	var ingest, sourcemap, agentConfig, json bool
	short := `Check if a "credentials" string has the given privilege(s)`
	long := short + `.
If no privilege(s) are specified, the credentials will be queried for all.`
	verify := &cobra.Command{
		Use:   "verify",
		Short: short,
		Long:  long,
		Run: makeAPIKeyRun(settings, &json, func(client es.Client, config *config.Config, args []string) error {
			privileges := booleansToPrivileges(ingest, sourcemap, agentConfig)
			if len(privileges) == 0 {
				// can't use "*" for querying
				privileges = auth.ActionsAll()
			}
			return verifyAPIKey(config, privileges, credentials, json)
		}),
	}
	verify.Flags().StringVar(&credentials, "credentials", "", `credentials for which check privileges (required)`)
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
	verify.MarkFlagRequired("credentials")
	verify.Flags().SortFlags = false

	return verify
}

type apikeyRunFunc func(client es.Client, config *config.Config, args []string) error

type cobraRunFunc func(cmd *cobra.Command, args []string)

func makeAPIKeyRun(settings instance.Settings, json *bool, f apikeyRunFunc) cobraRunFunc {
	return func(cmd *cobra.Command, args []string) {
		client, config, err := bootstrap(settings)
		if err != nil {
			printErr(err, *json)
			os.Exit(1)
		}
		if err := f(client, config, args); err != nil {
			printErr(err, *json)
			os.Exit(1)
		}
	}
}

// apm-server.api_key.enabled is implicitly true
func bootstrap(settings instance.Settings) (es.Client, *config.Config, error) {
	settings.ConfigOverrides = append(settings.ConfigOverrides, cfgfile.ConditionalOverride{
		Check: func(_ *common.Config) bool {
			return true
		},
		Config: common.MustNewConfigFrom(map[string]interface{}{
			"apm-server": map[string]interface{}{
				"api_key": map[string]interface{}{
					"enabled": true,
				},
			},
		}),
	})

	beat, err := instance.NewInitializedBeat(settings)
	if err != nil {
		return nil, nil, err
	}

	cfg, err := beat.BeatConfig()
	if err != nil {
		return nil, nil, err
	}

	var esOutputCfg *common.Config
	if beat.Config.Output.Name() == "elasticsearch" {
		esOutputCfg = beat.Config.Output.Config()
	}
	beaterConfig, err := config.NewConfig(cfg, esOutputCfg)
	if err != nil {
		return nil, nil, err
	}

	client, err := es.NewClient(beaterConfig.APIKeyConfig.ESConfig)
	if err != nil {
		return nil, nil, err
	}
	return client, beaterConfig, nil
}

func booleansToPrivileges(ingest, sourcemap, agentConfig bool) []es.PrivilegeAction {
	privileges := make([]es.PrivilegeAction, 0)
	if ingest {
		privileges = append(privileges, auth.PrivilegeEventWrite.Action)
	}
	if sourcemap {
		privileges = append(privileges, auth.PrivilegeSourcemapWrite.Action)
	}
	if agentConfig {
		privileges = append(privileges, auth.PrivilegeAgentConfigRead.Action)
	}
	return privileges
}

func createAPIKey(client es.Client, keyName, expiry string, privileges []es.PrivilegeAction, asJSON bool) error {

	// Elasticsearch will allow a user without the right apm privileges to create API keys, but the keys won't validate
	// check first whether the user has the right privileges, and bail out early if not
	// is not possible to always do it automatically, because file-based users and roles are not queryable
	hasPrivileges, err := es.HasPrivileges(context.Background(), client, es.HasPrivilegesRequest{
		Applications: []es.Application{
			{
				Name:       auth.Application,
				Privileges: privileges,
				Resources:  []es.Resource{auth.ResourceInternal},
			},
		},
	}, "")
	if err != nil {
		return err
	}
	if !hasPrivileges.HasAll {
		var missingPrivileges []string
		for action, hasPrivilege := range hasPrivileges.Application[auth.Application][auth.ResourceInternal] {
			if !hasPrivilege {
				missingPrivileges = append(missingPrivileges, string(action))
			}
		}
		return fmt.Errorf(`%s is missing the following requested privilege(s): %s.

You might try with the superuser, or add the APM application privileges to the role of the authenticated user, eg.:
PUT /_security/role/my_role {
	...
	"applications": [{
	  "application": "apm",
	  "privileges": ["sourcemap:write", "event:write", "config_agent:read"],
	  "resources": ["*"]
	}],
	...
}
		`, hasPrivileges.Username, strings.Join(missingPrivileges, ", "))
	}

	printText, printJSON := printers(asJSON)

	apikeyRequest := es.CreateAPIKeyRequest{
		Name: keyName,
		RoleDescriptors: es.RoleDescriptor{
			auth.Application: es.Applications{
				Applications: []es.Application{
					{
						Name:       auth.Application,
						Privileges: privileges,
						Resources:  []es.Resource{auth.ResourceAny},
					},
				},
			},
		},
	}
	if expiry != "" {
		apikeyRequest.Expiration = &expiry
	}

	response, err := es.CreateAPIKey(context.Background(), client, apikeyRequest)
	if err != nil {
		return err
	}

	type APIKey struct {
		es.CreateAPIKeyResponse
		Credentials string `json:"credentials"`
	}
	apikey := APIKey{
		CreateAPIKeyResponse: response,
		Credentials:          base64.StdEncoding.EncodeToString([]byte(response.ID + ":" + response.Key)),
	}

	printText("API Key created:")
	printText("")
	printText("Name ........... %s", apikey.Name)
	printText("Expiration ..... %s", humanTime(apikey.ExpirationMs))
	printText("Id ............. %s", apikey.ID)
	printText("API Key ........ %s (won't be shown again)", apikey.Key)
	printText(`Credentials .... %s (use it as "Authorization: APIKey <credentials>" header to communicate with APM Server, won't be shown again)`, apikey.Credentials)

	printJSON(apikey)
	return nil
}

func getAPIKey(client es.Client, id, name *string, validOnly, asJSON bool) error {
	if isSet(id) {
		name = nil
	} else if isSet(name) {
		id = nil
	}
	request := es.GetAPIKeyRequest{
		APIKeyQuery: es.APIKeyQuery{
			ID:   id,
			Name: name,
		},
	}

	apikeys, err := es.GetAPIKeys(context.Background(), client, request)
	if err != nil {
		return err
	}

	transform := es.GetAPIKeyResponse{APIKeys: make([]es.APIKeyResponse, 0)}
	printText, printJSON := printers(asJSON)
	for _, apikey := range apikeys.APIKeys {
		expiry := humanTime(apikey.ExpirationMs)
		if validOnly && (apikey.Invalidated || expiry == "expired") {
			continue
		}
		creation := time.Unix(apikey.Creation/1000, 0).Format("2006-02-01 15:04")
		printText("Username ....... %s", apikey.Username)
		printText("Api Key Name ... %s", apikey.Name)
		printText("Id ............. %s", apikey.ID)
		printText("Creation ....... %s", creation)
		printText("Invalidated .... %t", apikey.Invalidated)
		if !apikey.Invalidated {
			printText("Expiration ..... %s", expiry)
		}
		printText("")
		transform.APIKeys = append(transform.APIKeys, apikey)
	}
	printText("%d API Keys found", len(transform.APIKeys))
	printJSON(transform)
	return nil
}

func invalidateAPIKey(client es.Client, id, name *string, asJSON bool) error {
	if isSet(id) {
		name = nil
	} else if isSet(name) {
		id = nil
	}
	invalidateKeysRequest := es.InvalidateAPIKeyRequest{
		APIKeyQuery: es.APIKeyQuery{
			ID:   id,
			Name: name,
		},
	}
	invalidation, err := es.InvalidateAPIKey(context.Background(), client, invalidateKeysRequest)
	if err != nil {
		return err
	}

	printText, printJSON := printers(asJSON)
	printText("Invalidated keys ... %s", strings.Join(invalidation.Invalidated, ", "))
	printText("Error count ........ %d", invalidation.ErrorCount)
	printJSON(invalidation)
	return nil
}

func verifyAPIKey(config *config.Config, privileges []es.PrivilegeAction, credentials string, asJSON bool) error {
	perms := make(es.Permissions)
	printText, printJSON := printers(asJSON)
	for _, privilege := range privileges {
		builder, err := auth.NewBuilder(config)
		if err != nil {
			return err
		}
		result, err := builder.
			ForPrivilege(privilege).
			AuthorizationFor(headers.APIKey, credentials).
			AuthorizedFor(context.Background(), auth.ResourceInternal)
		if err != nil {
			return err
		}
		perms[privilege] = result.Authorized
		printText("Authorized for %s...: %s", humanPrivilege(privilege), humanBool(result.Authorized))
	}
	printJSON(perms)
	return nil
}

func humanBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func humanPrivilege(privilege es.PrivilegeAction) string {
	switch privilege {
	case auth.ActionAny:
		return fmt.Sprintf("all privileges (\"%v\")", privilege)
	default:
		return fmt.Sprintf("privilege \"%v\"", privilege)
	}
}

func humanTime(millis *int64) string {
	if millis == nil {
		return "never"
	}
	expiry := time.Unix(*millis/1000, 0)
	if !expiry.After(time.Now()) {
		return "expired"
	}
	return humanize.Time(expiry)
}

// returns 2 printers, one for text and one for JSON
// one of them will be a noop based on the boolean argument
func printers(b bool) (func(string, ...interface{}), func(interface{})) {
	var w1 io.Writer = os.Stdout
	var w2 = ioutil.Discard
	if b {
		w1 = ioutil.Discard
		w2 = os.Stdout
	}
	return func(f string, i ...interface{}) {
			fmt.Fprintf(w1, f, i...)
			fmt.Fprintln(w1)
		}, func(i interface{}) {
			data, err := json.MarshalIndent(i, "", "\t")
			if err != nil {
				fmt.Fprintln(w2, err)
			}
			fmt.Fprintln(w2, string(data))
		}
}

// prints an Elasticsearch error to stderr
func printErr(err error, asJSON bool) {
	if asJSON {
		var data []byte
		var m map[string]interface{}
		e := json.Unmarshal([]byte(err.Error()), &m)
		if e == nil {
			// err.Error() has JSON shape, likely coming from Elasticsearch
			data, _ = json.MarshalIndent(m, "", "\t")
		} else {
			// err.Error() is a bare string, likely coming from apm-server
			data, _ = json.MarshalIndent(struct {
				Error string `json:"error"`
			}{
				Error: err.Error(),
			}, "", "\t")
		}
		fmt.Fprintln(os.Stderr, string(data))
	} else {
		fmt.Fprintln(os.Stderr, err.Error())
	}
}

func isSet(s *string) bool {
	return s != nil && *s != ""
}
