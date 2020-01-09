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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/elastic/beats/libbeat/cfgfile"
	"github.com/elastic/beats/libbeat/cmd/instance"
	"github.com/elastic/beats/libbeat/common"

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
Most operations require the "manage_security" cluster privilege. Ensure to configure "apm-server.api_key.*" or 
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
		// always need to return error for possible scripts checking the exit code,
		// but printing the error must be done inside
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := bootstrap(settings)
			if err != nil {
				return err
			}
			privileges := booleansToPrivileges(ingest, sourcemap, agentConfig)
			if len(privileges) == 0 {
				privileges = []es.PrivilegeAction{auth.ActionAny}
			}
			createAPIKeyWithPrivileges(client, keyName, expiration, privileges, json)
			return nil
		},
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
	var purge, json bool
	short := "Invalidate API Key(s) by Id or Name"
	invalidate := &cobra.Command{
		Use:   "invalidate",
		Short: short,
		Long: short + `.
If both "id" and "name" are supplied, only "id" will be used.
If neither of them are, an error will be returned.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == "" && name == "" {
				return errors.New(`either "id" or "name" are required`)
			}
			client, _, err := bootstrap(settings)
			if err != nil {
				return err
			}
			invalidateAPIKey(client, &id, &name, purge, json)
			return nil
		},
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == "" && name == "" {
				return errors.New(`either "id" or "name" are required`)
			}
			client, _, err := bootstrap(settings)
			if err != nil {
				return err
			}
			getAPIKey(client, &id, &name, validOnly, json)
			return nil
		},
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
		RunE: func(cmd *cobra.Command, args []string) error {
			_, config, err := bootstrap(settings)
			if err != nil {
				return err
			}
			privileges := booleansToPrivileges(ingest, sourcemap, agentConfig)
			if len(privileges) == 0 {
				// can't use "*" for querying
				privileges = auth.ActionsAll()
			}
			verifyAPIKey(config, privileges, credentials, json)
			return nil
		},
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
	beaterConfig, err := config.NewConfig(beat.Info.Version, cfg, esOutputCfg)
	if err != nil {
		return nil, nil, err
	}

	var client es.Client
	if err == nil {
		client, err = es.NewClient(beaterConfig.APIKeyConfig.ESConfig)
	}

	return client, beaterConfig, err
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

// creates an API Key with the given privileges, *AND* all the privileges modeled in apm-server
// we need to ensure forward-compatibility, for which future privileges must be created here and
// during server startup because we don't know if customers will run this command
func createAPIKeyWithPrivileges(client es.Client, keyName, expiry string, privileges []es.PrivilegeAction, asJSON bool) {
	var privilegesRequest = make(es.CreatePrivilegesRequest)
	event := auth.PrivilegeEventWrite
	agentConfig := auth.PrivilegeAgentConfigRead
	sourcemap := auth.PrivilegeSourcemapWrite
	privilegesRequest[auth.Application] = map[es.PrivilegeName]es.Actions{
		agentConfig.Name: {Actions: []es.PrivilegeAction{agentConfig.Action}},
		event.Name:       {Actions: []es.PrivilegeAction{event.Action}},
		sourcemap.Name:   {Actions: []es.PrivilegeAction{sourcemap.Action}},
	}

	privilegesCreated, err := es.CreatePrivileges(client, privilegesRequest)

	if err != nil {
		printErr(err, asJSON)
		return
	}

	// Elasticsearch will allow a user without the right apm privileges to create API keys, but the keys won't validate
	// check first whether the user has the right privileges, and bail out early if not
	// is not possible to always do it automatically, because file-based users and roles are not queryable
	hasPrivileges, err := es.HasPrivileges(client, es.HasPrivilegesRequest{
		Applications: []es.Application{
			{
				Name:       auth.Application,
				Privileges: auth.ActionsAll(),
				Resources:  []es.Resource{auth.ResourceInternal},
			},
		},
	}, "")
	if err != nil {
		printErr(err, asJSON)
		return
	}
	if !hasPrivileges.HasAll {
		printErr(errors.New(fmt.Sprintf(`%s does not have privileges to create API keys.
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
		`, hasPrivileges.Username)), asJSON)
		return
	}

	printText, printJSON := printers(asJSON)
	for privilege, result := range privilegesCreated[auth.Application] {
		if result.Created {
			printText("Security privilege \"%v\" created", privilege)
		}
	}

	apikeyRequest := es.CreateApiKeyRequest{
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

	apikey, err := es.CreateAPIKey(client, apikeyRequest)
	if err != nil {
		printErr(err, asJSON)
		return
	}
	credentials := base64.StdEncoding.EncodeToString([]byte(apikey.Id + ":" + apikey.Key))
	apikey.Credentials = &credentials
	printText("API Key created:")
	printText("")
	printText("Name ........... %s", apikey.Name)
	printText("Expiration ..... %s", humanTime(apikey.ExpirationMs))
	printText("Id ............. %s", apikey.Id)
	printText("API Key ........ %s (won't be shown again)", apikey.Key)
	printText(`Credentials .... %s (use it as "Authorization: ApiKey <credentials>" header to communicate with APM Server, won't be shown again)`,
		credentials)

	printJSON(struct {
		es.CreateApiKeyResponse
		Privileges es.CreatePrivilegesResponse `json:"created_privileges,omitempty"`
	}{
		CreateApiKeyResponse: apikey,
		Privileges:           privilegesCreated,
	})
}

func getAPIKey(client es.Client, id, name *string, validOnly, asJSON bool) {
	if isSet(id) {
		name = nil
	} else if isSet(name) {
		id = nil
	}
	request := es.GetApiKeyRequest{
		ApiKeyQuery: es.ApiKeyQuery{
			Id:   id,
			Name: name,
		},
	}

	apikeys, err := es.GetAPIKeys(client, request)
	if err != nil {
		printErr(err, asJSON)
		return
	}

	transform := es.GetApiKeyResponse{ApiKeys: make([]es.ApiKeyResponse, 0)}
	printText, printJSON := printers(asJSON)
	for _, apikey := range apikeys.ApiKeys {
		expiry := humanTime(apikey.ExpirationMs)
		if validOnly && (apikey.Invalidated || expiry == "expired") {
			continue
		}
		creation := time.Unix(apikey.Creation/1000, 0).Format("2006-02-01 15:04")
		printText("Username ....... %s", apikey.Username)
		printText("Api Key Name ... %s", apikey.Name)
		printText("Id ............. %s", apikey.Id)
		printText("Creation ....... %s", creation)
		printText("Invalidated .... %t", apikey.Invalidated)
		if !apikey.Invalidated {
			printText("Expiration ..... %s", expiry)
		}
		printText("")
		transform.ApiKeys = append(transform.ApiKeys, apikey)
	}
	printText("%d API Keys found", len(transform.ApiKeys))
	printJSON(transform)
}

func invalidateAPIKey(client es.Client, id, name *string, deletePrivileges, asJSON bool) {
	if isSet(id) {
		name = nil
	} else if isSet(name) {
		id = nil
	}
	invalidateKeysRequest := es.InvalidateApiKeyRequest{
		ApiKeyQuery: es.ApiKeyQuery{
			Id:   id,
			Name: name,
		},
	}

	invalidation, err := es.InvalidateAPIKey(client, invalidateKeysRequest)
	if err != nil {
		printErr(err, asJSON)
		return
	}
	printText, printJSON := printers(asJSON)
	out := struct {
		es.InvalidateApiKeyResponse
		Privileges []es.DeletePrivilegeResponse `json:"deleted_privileges,omitempty"`
	}{
		InvalidateApiKeyResponse: invalidation,
		Privileges:               make([]es.DeletePrivilegeResponse, 0),
	}
	printText("Invalidated keys ... %s", strings.Join(invalidation.Invalidated, ", "))
	printText("Error count ........ %d", invalidation.ErrorCount)

	for _, privilege := range auth.PrivilegesAll {
		if !deletePrivileges {
			break
		}
		deletePrivilegesRequest := es.DeletePrivilegeRequest{
			Application: auth.Application,
			Privilege:   privilege.Name,
		}

		deletion, err := es.DeletePrivileges(client, deletePrivilegesRequest)
		if err != nil {
			continue
		}
		if _, ok := deletion[auth.Application]; !ok {
			continue
		}
		if result, ok := deletion[auth.Application][privilege.Name]; ok && result.Found {
			printText("Deleted privilege \"%v\"", privilege)
		}
		out.Privileges = append(out.Privileges, deletion)
	}
	printJSON(out)
}

func verifyAPIKey(config *config.Config, privileges []es.PrivilegeAction, credentials string, asJSON bool) {
	perms := make(es.Permissions)

	printText, printJSON := printers(asJSON)
	var err error

	for _, privilege := range privileges {
		var builder *auth.Builder
		builder, err := auth.NewBuilder(*config)
		if err != nil {
			break
		}

		var authorized bool
		authorized, err = builder.
			ForPrivilege(privilege).
			AuthorizationFor(headers.APIKey, credentials).
			AuthorizedFor(auth.ResourceInternal)
		if err != nil {
			break
		}

		perms[privilege] = authorized
		printText("Authorized for %s...: %s", humanPrivilege(privilege), humanBool(authorized))
	}

	if err != nil {
		printErr(err, asJSON)
	} else {
		printJSON(perms)
	}
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
		return fmt.Sprint("never")
	}
	seconds := time.Until(time.Unix(*millis/1000, 0)).Seconds()
	if seconds < 0 {
		return fmt.Sprintf("expired")
	}
	minutes := math.Round(seconds / 60)
	if minutes < 2 {
		return fmt.Sprintf("%.0f seconds", seconds)
	}
	hours := math.Round(minutes / 60)
	if hours < 2 {
		return fmt.Sprintf("%.0f minutes", minutes)
	}
	days := math.Round(hours / 24)
	if days < 2 {
		return fmt.Sprintf("%.0f hours", hours)
	}
	years := math.Round(days / 365)
	if years < 2 {
		return fmt.Sprintf("%.0f days", days)
	}
	return fmt.Sprintf("%.0f years", years)
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
