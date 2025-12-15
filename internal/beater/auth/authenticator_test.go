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
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func TestAuthenticatorNone(t *testing.T) {
	authenticator, err := NewAuthenticator(config.AgentAuth{}, noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)

	// If the server has no configured auth methods, all requests are allowed.
	for _, kind := range []string{"", headers.APIKey, headers.Bearer} {
		details, authz, err := authenticator.Authenticate(context.Background(), kind, "")
		require.NoError(t, err)
		assert.Equal(t, AuthenticationDetails{Method: MethodNone}, details)
		assert.Equal(t, allowAuth{}, authz)
	}
}

func TestAuthenticatorAuthRequired(t *testing.T) {
	withSecretToken := config.AgentAuth{SecretToken: "secret_token"}
	withAPIKey := config.AgentAuth{
		APIKey: config.APIKeyAgentAuth{Enabled: true, ESConfig: elasticsearch.DefaultConfig()},
	}
	for _, cfg := range []config.AgentAuth{withSecretToken, withAPIKey} {
		authenticator, err := NewAuthenticator(cfg, noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))
		require.NoError(t, err)

		details, authz, err := authenticator.Authenticate(context.Background(), "", "")
		assert.Error(t, err)
		assert.EqualError(t, err, "authentication failed: missing or improperly formatted Authorization header: expected 'Authorization: Bearer secret_token' or 'Authorization: ApiKey base64(API key ID:API key)'")
		assert.True(t, errors.Is(err, ErrAuthFailed))
		assert.Zero(t, details)
		assert.Nil(t, authz)

		details, authz, err = authenticator.Authenticate(context.Background(), "magic", "")
		assert.Error(t, err)
		assert.EqualError(t, err, `authentication failed: unknown Authentication header magic: expected 'Authorization: Bearer secret_token' or 'Authorization: ApiKey base64(API key ID:API key)'`)
		assert.True(t, errors.Is(err, ErrAuthFailed))
		assert.Zero(t, details)
		assert.Nil(t, authz)
	}
}

func TestAuthenticatorSecretToken(t *testing.T) {
	authenticator, err := NewAuthenticator(config.AgentAuth{SecretToken: "valid"}, noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)

	details, authz, err := authenticator.Authenticate(context.Background(), headers.Bearer, "invalid")
	assert.Equal(t, ErrAuthFailed, err)
	assert.Zero(t, details)
	assert.Nil(t, authz)

	details, authz, err = authenticator.Authenticate(context.Background(), headers.Bearer, "valid")
	assert.NoError(t, err)
	assert.Equal(t, AuthenticationDetails{Method: MethodSecretToken}, details)
	assert.Equal(t, allowAuth{}, authz)
}

func TestAuthenticatorAPIKey(t *testing.T) {
	var requestURLPath string
	var requestBody []byte
	var requestAuthorizationHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURLPath = r.URL.Path
		requestBody, _ = io.ReadAll(r.Body)
		requestAuthorizationHeader = r.Header.Get("Authorization")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Write([]byte(`{
                        "username": "api_key_username",
			"application": {
				"apm": {
					"-": {"config_agent:read": true, "event:write": true, "sourcemap:write": false}
				}
			}
		}`))
	}))
	defer srv.Close()

	esConfig := elasticsearch.DefaultConfig()
	esConfig.Hosts = elasticsearch.Hosts{srv.URL}
	authenticator, err := NewAuthenticator(config.AgentAuth{
		APIKey: config.APIKeyAgentAuth{Enabled: true, LimitPerMin: 100, ESConfig: esConfig},
	}, noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)

	credentials := base64.StdEncoding.EncodeToString([]byte("id_value:key_value"))
	details, authz, err := authenticator.Authenticate(context.Background(), headers.APIKey, credentials)
	assert.NoError(t, err)
	assert.Equal(t, AuthenticationDetails{
		Method: MethodAPIKey,
		APIKey: &APIKeyAuthenticationDetails{
			ID:       "id_value",
			Username: "api_key_username",
		},
	}, details)
	assert.Equal(t, &apikeyAuthorizer{permissions: elasticsearch.Permissions{
		"config_agent:read": true,
		"event:write":       true,
		"sourcemap:write":   false,
	}}, authz)

	assert.Equal(t, "/_security/user/_has_privileges", requestURLPath)
	assert.Equal(t, `{"application":[{"application":"apm","privileges":["config_agent:read","event:write","sourcemap:write"],"resources":["-"]}]}`, string(requestBody))
	assert.Equal(t, "ApiKey "+credentials, requestAuthorizationHeader)
}

func TestAuthenticatorAPIKeyErrors(t *testing.T) {
	esConfig := elasticsearch.DefaultConfig()
	esConfig.Hosts = elasticsearch.Hosts{"testing.invalid"}
	esConfig.Backoff.Init = time.Nanosecond
	esConfig.Backoff.Max = time.Nanosecond
	authenticator, err := NewAuthenticator(config.AgentAuth{
		APIKey: config.APIKeyAgentAuth{Enabled: true, LimitPerMin: 100, ESConfig: esConfig},
	}, noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)

	// Make sure that we can't auth with an empty secret token if secret token auth is not configured, but API Key auth is.
	details, authz, err := authenticator.Authenticate(context.Background(), headers.Bearer, "")
	assert.Equal(t, ErrAuthFailed, err)
	assert.Zero(t, details)
	assert.Nil(t, authz)

	details, authz, err = authenticator.Authenticate(context.Background(), headers.APIKey, "invalid_base64")
	assert.EqualError(t, err, "authentication failed: improperly encoded ApiKey credentials: expected base64(ID:APIKey): illegal base64 data at input byte 7")
	assert.True(t, errors.Is(err, ErrAuthFailed))
	assert.Zero(t, details)
	assert.Nil(t, authz)

	credentials := base64.StdEncoding.EncodeToString([]byte("malformatted_credentials"))
	details, authz, err = authenticator.Authenticate(context.Background(), headers.APIKey, credentials)
	assert.EqualError(t, err, "authentication failed: improperly formatted ApiKey credentials: expected base64(ID:APIKey)")
	assert.True(t, errors.Is(err, ErrAuthFailed))
	assert.Zero(t, details)
	assert.Nil(t, authz)

	credentials = base64.StdEncoding.EncodeToString([]byte("id_value:key_value"))
	details, authz, err = authenticator.Authenticate(context.Background(), headers.APIKey, credentials)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, ErrAuthFailed)) // failure to communicate with elastiscsearch is *not* an auth failure
	assert.Zero(t, details)
	assert.Nil(t, authz)

	responseStatusCode := http.StatusUnauthorized
	responseBody := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(responseStatusCode)
		w.Write([]byte(responseBody))
	}))
	defer srv.Close()
	esConfig.Hosts = elasticsearch.Hosts{srv.URL}
	authenticator, err = NewAuthenticator(config.AgentAuth{
		APIKey: config.APIKeyAgentAuth{Enabled: true, LimitPerMin: 2, ESConfig: esConfig},
	}, noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	details, authz, err = authenticator.Authenticate(context.Background(), headers.APIKey, credentials)
	assert.Equal(t, ErrAuthFailed, err)
	assert.Zero(t, details)
	assert.Nil(t, authz)

	// API Key is valid, but grants none of the requested privileges.
	responseStatusCode = http.StatusOK
	responseBody = `{
            "application": {
              "apm": {
                "-": {"config_agent:read": false, "event:write": false, "sourcemap:write": false}
              }
            }
        }`
	defer srv.Close()
	esConfig.Hosts = elasticsearch.Hosts{srv.URL}
	authenticator, err = NewAuthenticator(config.AgentAuth{
		APIKey: config.APIKeyAgentAuth{Enabled: true, LimitPerMin: 100, ESConfig: esConfig},
	}, noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	details, authz, err = authenticator.Authenticate(context.Background(), headers.APIKey, credentials)
	assert.Equal(t, ErrAuthFailed, err)
	assert.Zero(t, details)
	assert.Nil(t, authz)
}

func TestAuthenticatorAPIKeyCache(t *testing.T) {
	validCredentials := base64.StdEncoding.EncodeToString([]byte("valid_id:key_value"))
	validCredentials2 := base64.StdEncoding.EncodeToString([]byte("valid_id:key_value_2"))
	invalidCredentials := base64.StdEncoding.EncodeToString([]byte("invalid_id:key_value"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		credentials := strings.Fields(r.Header.Get("Authorization"))[1]
		switch credentials {
		case validCredentials:
			w.Header().Set("X-Elastic-Product", "Elasticsearch")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
                          "username": "api_key_username",
                          "application": {
                            "apm": {
			      "-": {"config_agent:read": true, "event:write": true, "sourcemap:write": false}
                            }
                          }
		       }`))
		case invalidCredentials:
			w.WriteHeader(http.StatusUnauthorized)
		default:
			panic("unexpected credentials: " + credentials)
		}
	}))
	defer srv.Close()

	exporter := &manualExporter{}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)))

	esConfig := elasticsearch.DefaultConfig()
	esConfig.Hosts = elasticsearch.Hosts{srv.URL}
	apikeyAuthConfig := config.APIKeyAgentAuth{Enabled: true, LimitPerMin: 2, ESConfig: esConfig}
	authenticator, err := NewAuthenticator(config.AgentAuth{APIKey: apikeyAuthConfig}, tp, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)

	for i := 0; i < apikeyAuthConfig.LimitPerMin+1; i++ {
		_, _, err := authenticator.Authenticate(context.Background(), headers.APIKey, validCredentials)
		assert.NoError(t, err)
	}
	spans := exporter.payloads()
	assert.Len(t, spans, 1)
	assert.Equal(t, "Elasticsearch: GET _security/user/_has_privileges", spans[0].Name())
	exporter.clear()

	// API Key checks are cached based on the API Key ID, not the full credential.
	_, _, err = authenticator.Authenticate(context.Background(), headers.APIKey, validCredentials2)
	assert.NoError(t, err)
	assert.Len(t, exporter.payloads(), 0)
	exporter.clear()

	for i := 0; i < apikeyAuthConfig.LimitPerMin+1; i++ {
		_, _, err = authenticator.Authenticate(context.Background(), headers.APIKey, invalidCredentials)
		assert.Equal(t, ErrAuthFailed, err)
	}
	assert.Len(t, exporter.payloads(), 1)

	credentials := base64.StdEncoding.EncodeToString([]byte("id_value3:key_value"))
	_, _, err = authenticator.Authenticate(context.Background(), headers.APIKey, credentials)
	assert.EqualError(t, err, "api_key limit reached, check your logs for failed authorization attempts or consider increasing config option `apm-server.api_key.limit`")
}

func TestAuthenticatorAnonymous(t *testing.T) {
	// Anonymous access is only effective when some other auth method is enabled.
	authenticator, err := NewAuthenticator(config.AgentAuth{
		Anonymous: config.AnonymousAgentAuth{Enabled: true},
	}, noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	details, authz, err := authenticator.Authenticate(context.Background(), "", "")
	assert.NoError(t, err)
	assert.Equal(t, AuthenticationDetails{Method: MethodNone}, details)
	assert.Equal(t, allowAuth{}, authz)

	authenticator, err = NewAuthenticator(config.AgentAuth{
		SecretToken: "secret_token",
		Anonymous:   config.AnonymousAgentAuth{Enabled: true},
	}, noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	details, authz, err = authenticator.Authenticate(context.Background(), "", "")
	assert.NoError(t, err)
	assert.Equal(t, AuthenticationDetails{Method: MethodAnonymous}, details)
	assert.Equal(t, newAnonymousAuth(nil, nil), authz)
}

type manualExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (e *manualExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, spans...)
	return nil
}

func (e *manualExporter) Shutdown(ctx context.Context) error {
	return nil
}

func (e *manualExporter) payloads() []sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.spans
}

func (e *manualExporter) clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = nil
}
