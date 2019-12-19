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
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"

	auth "github.com/elastic/apm-server/beater/authorization"
	es "github.com/elastic/apm-server/elasticsearch"
)

// creates an API Key with the given privileges, *AND* all the privileges modeled in apm-server
// we need to ensure forward-compatibility, for which future privileges must be created here and
// during server startup because we don't know if customers will run this command
func createApiKeyWithPrivileges(client es.Client, apikeyName, expiry string, privileges []es.Privilege, asJson bool) error {
	var privilegesRequest = make(es.CreatePrivilegesRequest)
	event := auth.PrivilegeEventWrite
	agentConfig := auth.PrivilegeAgentConfigRead
	sourcemap := auth.PrivilegeSourcemapWrite
	privilegesRequest[auth.Application] = map[es.PrivilegeName]es.Actions{
		agentConfig.Name: {Actions: []es.Privilege{agentConfig.Action}},
		event.Name:       {Actions: []es.Privilege{event.Action}},
		sourcemap.Name:   {Actions: []es.Privilege{sourcemap.Action}},
	}

	privilegesCreated, err := es.CreatePrivileges(client, privilegesRequest)

	if err != nil {
		return printErr(err,
			`Error creating privileges for APM Server, do you have the "manage_cluster" security privilege?`,
			asJson)
	}

	printText, printJson := printers(asJson)
	for privilege, result := range privilegesCreated[auth.Application] {
		if result.Created {
			printText("Security privilege \"%v\" created", privilege)
		}
	}

	apikeyRequest := es.CreateApiKeyRequest{
		Name: apikeyName,
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
		return printErr(err, fmt.Sprintf(
			`Error creating the API Key %s, do you have the "manage_cluster" security privilege?`, apikeyName),
			asJson)
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

	return printJson(struct {
		es.CreateApiKeyResponse
		Privileges es.CreatePrivilegesResponse `json:"created_privileges,omitempty"`
	}{
		CreateApiKeyResponse: apikey,
		Privileges:           privilegesCreated,
	})
}

func getApiKey(client es.Client, id, name *string, validOnly, asJson bool) error {
	if isSet(id) {
		name = nil
	} else if !(isSet(id) || isSet(name)) {
		return printErr(errors.New("could not query Elasticsearch"),
			`either "id" or "name" are required`,
			asJson)
	}
	request := es.GetApiKeyRequest{
		ApiKeyQuery: es.ApiKeyQuery{
			Id:   id,
			Name: name,
		},
	}

	apikeys, err := es.GetAPIKeys(client, request)
	if err != nil {
		return printErr(err,
			`Error retrieving API Key(s) for APM Server, do you have the "manage_cluster" security privilege?`,
			asJson)
	}

	transform := es.GetApiKeyResponse{ApiKeys: make([]es.ApiKeyResponse, 0)}
	printText, printJson := printers(asJson)
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
		printText("Expiration ..... %s", expiry)
		printText("")
		transform.ApiKeys = append(transform.ApiKeys, apikey)
	}
	printText("%d API Keys found", len(transform.ApiKeys))
	return printJson(transform)
}

func invalidateApiKey(client es.Client, id, name *string, deletePrivileges, asJson bool) error {
	if isSet(id) {
		name = nil
	} else if !(isSet(id) || isSet(name)) {
		return printErr(errors.New("could not query Elasticsearch"),
			`either "id" or "name" are required`,
			asJson)
	}
	invalidateKeysRequest := es.InvalidateApiKeyRequest{
		ApiKeyQuery: es.ApiKeyQuery{
			Id:   id,
			Name: name,
		},
	}

	invalidation, err := es.InvalidateAPIKey(client, invalidateKeysRequest)
	if err != nil {
		return printErr(err,
			`Error invalidating API Key(s), do you have the "manage_cluster" security privilege?`,
			asJson)
	}
	printText, printJson := printers(asJson)
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
	return printJson(out)
}

func verifyApiKey(config *config.Config, privileges []es.Privilege, credentials string, asJson bool) error {
	perms := make(es.Perms)

	printText, printJson := printers(asJson)
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
		return printErr(err, "could not verify credentials, please check your Elasticsearch connection", asJson)
	}
	return printJson(perms)
}

func humanBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func humanPrivilege(privilege es.Privilege) string {
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
func printers(b bool) (func(string, ...interface{}), func(interface{}) error) {
	var w1 io.Writer = os.Stdout
	var w2 = ioutil.Discard
	if b {
		w1 = ioutil.Discard
		w2 = os.Stdout
	}
	return func(f string, i ...interface{}) {
			fmt.Fprintf(w1, f, i...)
			fmt.Fprintln(w1)
		}, func(i interface{}) error {
			data, err := json.MarshalIndent(i, "", "\t")
			fmt.Fprintln(w2, string(data))
			// conform the interface
			return errors.Wrap(err, fmt.Sprintf("%v+", i))
		}
}

// prints an Elasticsearch error to stderr, with some additional contextual information as a hint
func printErr(err error, help string, asJson bool) error {
	if asJson {
		var data []byte
		var m map[string]interface{}
		e := json.Unmarshal([]byte(err.Error()), &m)
		if e == nil {
			// err.Error() has JSON shape, likely coming from Elasticsearch
			m["help"] = help
			data, _ = json.MarshalIndent(m, "", "\t")
		} else {
			// err.Error() is a bare string, likely coming from apm-server
			data, _ = json.MarshalIndent(struct {
				Error string `json:"error"`
				Help  string `json:"help"`
			}{
				Error: err.Error(),
				Help:  help,
			}, "", "\t")
		}
		fmt.Fprintln(os.Stderr, string(data))
	} else {
		fmt.Fprintln(os.Stderr, help)
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return errors.Wrap(err, help)
}

func isSet(s *string) bool {
	return s != nil && *s != ""
}
