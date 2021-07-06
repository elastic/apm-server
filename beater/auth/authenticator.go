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

package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/elasticsearch"
)

// Method identifies an authentication and authorization method.
type Method string

const (
	// MethodNone is used when the server has no auth methods configured,
	// meaning access is entirely unrestricted to unauthenticated clients.
	//
	// This exists to differentiate from allowed unauthenticated/anonymous
	// access when the server has other auth methods defined.
	MethodNone Method = "none"

	// MethodAPIKey identifies the auth method using Elasticsearch API Keys.
	// Clients that authenticate with an API Key may have restricted privileges.
	MethodAPIKey Method = "api_key"

	// MethodSecretToken identifies the auth methd using a shared secret token.
	// Clients with this secret token have unrestricted privileges.
	MethodSecretToken Method = "secret_token"
)

// Action identifies an action to authorize.
type Action string

const (
	// ActionAgentConfig is an Action describing an attempt to read agent config.
	ActionAgentConfig Action = "agent_config"

	// ActionEventIngest is an Action describing an attempt to ingest events.
	ActionEventIngest Action = "event_ingest"

	// ActionSourcemapUpload is an Action describing an attempt to upload a source map.
	ActionSourcemapUpload Action = "sourcemap"
)

const (
	cacheTimeoutMinute       = 1 * time.Minute
	expectedAuthHeaderFormat = "expected 'Authorization: Bearer secret_token' or 'Authorization: ApiKey base64(API key ID:API key)'"
)

// ErrAuthFailed is an error returned by Authenticator.Authenticate to indicate
// that a client has failed to authenticate, for example by failing to provide
// credentials or by providing an invalid or expired API Key.
var ErrAuthFailed = errors.New("authentication failed")

var errAuthMissing = fmt.Errorf("%w: missing or improperly formatted Authorization header: %s", ErrAuthFailed, expectedAuthHeaderFormat)

// ErrUnauthorized is an error returned by Authorizer.Authorize to indicate that
// the client is unauthorized for some action and resource. This should be wrapped
// to provide a reason, and checked using `errors.Is`.
var ErrUnauthorized = errors.New("unauthorized")

// Authenticator authenticates clients.
type Authenticator struct {
	secretToken string

	apikey *apikeyAuth
}

// Authorizer provides an interface for authorizing an action and resource.
//
// TODO(axw) rename interface to Authorizer and method to Authorize.
type Authorizer interface {
	// Authorize checks if the client is authorized for the given action and
	// resource, returning ErrUnauthorized if it is not. Other errors may be
	// returned, for example because the server cannot communicate with
	// external systems.
	Authorize(context.Context, Action, Resource) error
}

// Resource holds parameters for restricting access that may be checked by
// Authorizer.Authorize.
type Resource struct {
	// AgentName holds the agent name associated with the agent making the
	// request. This may be empty if the agent is unknown or irrelevant,
	// such as in a request to the healthcheck endpoint.
	AgentName string

	// ServiceName holds the service name associated with the agent making
	// the request. This may be empty if the agent is unknown or irrelevant,
	// such as in a request to the healthcheck endpoint.
	ServiceName string
}

// AuthenticationDetails holds authentication details for a client.
type AuthenticationDetails struct {
	// Method holds the authentication kind used.
	//
	// Method will be empty for unauthenticated (anonymous) requests when
	// the server has at least one auth method defined. When the server
	// has no auth methods defined, this will be MethodNone.
	Method Method

	// APIKey holds authentication details related to API Key auth.
	// This will be set when Method is MethodAPIKey.
	APIKey *APIKeyAuthenticationDetails
}

// APIKeyAuthenticationDetails holds API Key related authentication details.
type APIKeyAuthenticationDetails struct {
	// ID holds the non-secret ID of the API Key.
	ID string

	// Username holds the username associated with the API Key.
	Username string
}

// NewAuthenticator creates an Authenticator with config, authenticating
// clients with one of the allowed methods.
func NewAuthenticator(cfg config.AgentAuth) (*Authenticator, error) {
	b := Authenticator{secretToken: cfg.SecretToken}
	if cfg.APIKey.Enabled {
		// Do not use apm-server's credentials for API Key requests;
		// we should only use API Key credentials provided by clients
		// to the Authenticate method.
		cfg.APIKey.ESConfig.Username = ""
		cfg.APIKey.ESConfig.Password = ""
		cfg.APIKey.ESConfig.APIKey = ""
		client, err := elasticsearch.NewClient(cfg.APIKey.ESConfig)
		if err != nil {
			return nil, err
		}

		cache := newPrivilegesCache(cacheTimeoutMinute, cfg.APIKey.LimitPerMin)
		b.apikey = newApikeyAuth(client, cache)
	}
	return &b, nil
}

// Authenticate authenticates a client given an authentication method and token,
// returning the authentication details and an Authorizer for authorizing specific
// actions and resources.
//
// Authenticate will return ErrAuthFailed (possibly wrapped) if at least one auth
// method is configured and no valid credentials have been supplied. Other errors
// may be returned, for example because the server cannot communicate with external
// systems.
func (a *Authenticator) Authenticate(ctx context.Context, kind string, token string) (AuthenticationDetails, Authorizer, error) {
	if a.apikey == nil && a.secretToken == "" {
		// No auth required, let everyone through.
		return AuthenticationDetails{Method: MethodNone}, allowAuth{}, nil
	}
	switch kind {
	case "":
		return AuthenticationDetails{}, nil, errAuthMissing
	case headers.APIKey:
		if a.apikey != nil {
			details, authz, err := a.apikey.authenticate(ctx, token)
			if err != nil {
				return AuthenticationDetails{}, nil, err
			}
			return AuthenticationDetails{Method: MethodAPIKey, APIKey: details}, authz, nil
		}
	case headers.Bearer:
		if a.secretToken != "" && subtle.ConstantTimeCompare([]byte(a.secretToken), []byte(token)) == 1 {
			return AuthenticationDetails{Method: MethodSecretToken}, allowAuth{}, nil
		}
	default:
		return AuthenticationDetails{}, nil, fmt.Errorf(
			"%w: unknown Authentication header %s: %s",
			ErrAuthFailed, kind, expectedAuthHeaderFormat,
		)
	}
	return AuthenticationDetails{}, nil, ErrAuthFailed
}
