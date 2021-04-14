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

package elasticsearch

import (
	"context"
	"net/http"

	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/elastic/go-elasticsearch/v7/esutil"
)

// CreateAPIKey requires manage_api_key cluster privilege
func CreateAPIKey(ctx context.Context, client Client, apikeyReq CreateAPIKeyRequest) (CreateAPIKeyResponse, error) {
	var apikey CreateAPIKeyResponse
	req := esapi.SecurityCreateAPIKeyRequest{Body: esutil.NewJSONReader(apikeyReq)}
	err := doRequest(ctx, client, req, &apikey)
	return apikey, err
}

// GetAPIKeys requires manage_api_key cluster privilege
func GetAPIKeys(ctx context.Context, client Client, apikeyReq GetAPIKeyRequest) (GetAPIKeyResponse, error) {
	req := esapi.SecurityGetAPIKeyRequest{}
	if apikeyReq.ID != nil {
		req.ID = *apikeyReq.ID
	} else if apikeyReq.Name != nil {
		req.Name = *apikeyReq.Name
	}
	var apikey GetAPIKeyResponse
	err := doRequest(ctx, client, req, &apikey)
	return apikey, err
}

// InvalidateAPIKey requires manage_api_key cluster privilege
func InvalidateAPIKey(ctx context.Context, client Client, apikeyReq InvalidateAPIKeyRequest) (InvalidateAPIKeyResponse, error) {
	var confirmation InvalidateAPIKeyResponse
	req := esapi.SecurityInvalidateAPIKeyRequest{Body: esutil.NewJSONReader(apikeyReq)}
	err := doRequest(ctx, client, req, &confirmation)
	return confirmation, err
}

func HasPrivileges(ctx context.Context, client Client, privileges HasPrivilegesRequest, credentials string) (HasPrivilegesResponse, error) {
	var info HasPrivilegesResponse
	req := esapi.SecurityHasPrivilegesRequest{Body: esutil.NewJSONReader(privileges)}
	if credentials != "" {
		header := make(http.Header)
		header.Set("Authorization", "ApiKey "+credentials)
		req.Header = header
	}
	err := doRequest(ctx, client, req, &info)
	return info, err
}

type CreateAPIKeyRequest struct {
	Name            string                 `json:"name"`
	Expiration      *string                `json:"expiration,omitempty"`
	RoleDescriptors RoleDescriptor         `json:"role_descriptors"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type CreateAPIKeyResponse struct {
	APIKey
	Key string `json:"api_key"`
}

type GetAPIKeyRequest struct {
	APIKeyQuery
	Owner bool `json:"owner"`
}

type GetAPIKeyResponse struct {
	APIKeys []APIKeyResponse `json:"api_keys"`
}

type HasPrivilegesRequest struct {
	// can't reuse the `Applications` type because here the JSON attribute must be singular
	Applications []Application `json:"application"`
}
type HasPrivilegesResponse struct {
	Username    string                             `json:"username"`
	HasAll      bool                               `json:"has_all_requested"`
	Application map[AppName]PermissionsPerResource `json:"application"`
}

type InvalidateAPIKeyRequest struct {
	// normally the Elasticsearch API will require either Ids or Name, but not both
	IDs  []string `json:"ids,omitempty"`
	Name *string  `json:"name,omitempty"`
}

type InvalidateAPIKeyResponse struct {
	Invalidated []string `json:"invalidated_api_keys"`
	ErrorCount  int      `json:"error_count"`
}

type RoleDescriptor map[AppName]Applications

type Applications struct {
	Applications []Application `json:"applications"`
}

type Application struct {
	Name       AppName           `json:"application"`
	Privileges []PrivilegeAction `json:"privileges"`
	Resources  []Resource        `json:"resources"`
}

type APIKeyResponse struct {
	APIKey
	Creation    int64                  `json:"creation"`
	Invalidated bool                   `json:"invalidated"`
	Username    string                 `json:"username"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type APIKeyQuery struct {
	// normally the Elasticsearch API will require either Id or Name, but not both
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

type APIKey struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ExpirationMs *int64 `json:"expiration,omitempty"`
}

type Permissions map[PrivilegeAction]bool

type PermissionsPerResource map[Resource]Permissions

type AppName string

type Resource string

// NamedPrivilege is a tuple consisting of a name and an action.
// In Elasticsearch a "privilege" represents both an "action" that a user might/might not have authorization to
// perform, and such a tuple.
// In apm-server, each name is associated with one action, but that needs not to be the case (see PrivilegeGroup)
type NamedPrivilege struct {
	Name   PrivilegeName
	Action PrivilegeAction
}

type PrivilegeAction string

type PrivilegeName string

func NewPrivilege(name, action string) NamedPrivilege {
	return NamedPrivilege{
		Name:   PrivilegeName(name),
		Action: PrivilegeAction(action),
	}
}
