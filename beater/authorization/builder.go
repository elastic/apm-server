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

package authorization

import (
	"time"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/elasticsearch"
)

// Builder creates an authorization Handler depending on configuration options
type Builder struct {
	apikey   *apikeyBuilder
	bearer   *bearerBuilder
	fallback Authorization
}

// Handler returns the authorization method according to provided information
type Handler Builder

// Authorization interface to be implemented by different auth types
type Authorization interface {
	AuthorizedFor(elasticsearch.Resource) (bool, error)
	IsAuthorizationConfigured() bool
}

const (
	cacheTimeoutMinute = 1 * time.Minute
)

// NewBuilder creates authorization builder based off of the given information
// if apm-server.api_key is enabled, authorization is granted/denied solely
// based on the request Authorization header
func NewBuilder(cfg *config.Config) (*Builder, error) {
	b := Builder{}
	b.fallback = AllowAuth{}
	if cfg.APIKeyConfig.IsEnabled() {
		// do not use username+password for API Key requests
		cfg.APIKeyConfig.ESConfig.Username = ""
		cfg.APIKeyConfig.ESConfig.Password = ""
		cfg.APIKeyConfig.ESConfig.APIKey = ""
		client, err := elasticsearch.NewClient(cfg.APIKeyConfig.ESConfig)
		if err != nil {
			return nil, err
		}

		cache := newPrivilegesCache(cacheTimeoutMinute, cfg.APIKeyConfig.LimitPerMin)
		b.apikey = newApikeyBuilder(client, cache, []elasticsearch.PrivilegeAction{})
		b.fallback = DenyAuth{}
	}
	if cfg.SecretToken != "" {
		b.bearer = &bearerBuilder{cfg.SecretToken}
		b.fallback = DenyAuth{}
	}
	b.fallback.IsAuthorizationConfigured()
	return &b, nil
}

// ForPrivilege creates an authorization Handler checking for this privilege
func (b *Builder) ForPrivilege(privilege elasticsearch.PrivilegeAction) *Handler {
	return b.ForAnyOfPrivileges(privilege)
}

// ForAnyOfPrivileges creates an authorization Handler checking for any of the provided privileges
func (b *Builder) ForAnyOfPrivileges(privileges ...elasticsearch.PrivilegeAction) *Handler {
	handler := Handler{bearer: b.bearer, fallback: b.fallback}
	if b.apikey != nil {
		handler.apikey = newApikeyBuilder(b.apikey.esClient, b.apikey.cache, privileges)
	}
	return &handler
}

// AuthorizationFor returns proper authorization implementation depending on the given kind, configured with the token.
func (h *Handler) AuthorizationFor(kind string, token string) Authorization {
	switch kind {
	case headers.APIKey:
		if h.apikey == nil {
			return h.fallback
		}
		return h.apikey.forKey(token)
	case headers.Bearer:
		if h.bearer == nil {
			return h.fallback
		}
		return h.bearer.forToken(token)
	default:
		return h.fallback
	}
}
