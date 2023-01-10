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

package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/request"
)

const (
	errMaxAgeDuration = 5 * time.Minute

	msgInvalidQuery       = "invalid query"
	msgMethodUnsupported  = "method not supported"
	msgServiceUnavailable = "service unavailable"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.DefaultMonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.acm")

	errCacheControl = fmt.Sprintf("max-age=%v, must-revalidate", errMaxAgeDuration.Seconds())
)

type handler struct {
	f agentcfg.Fetcher

	allowAnonymousAgents                    []string
	cacheControl, defaultServiceEnvironment string
}

func NewHandler(
	f agentcfg.Fetcher,
	cacheMaxAge time.Duration,
	defaultServiceEnvironment string,
	allowAnonymousAgents []string,
) request.Handler {
	if f == nil {
		panic("fetcher must not be nil")
	}
	cacheControl := fmt.Sprintf("max-age=%v, must-revalidate", cacheMaxAge.Seconds())
	h := &handler{
		f:                         f,
		cacheControl:              cacheControl,
		defaultServiceEnvironment: defaultServiceEnvironment,
		allowAnonymousAgents:      allowAnonymousAgents,
	}

	return h.Handle
}

// Handler implements request.Handler for managing agent central configuration
// requests.
func (h *handler) Handle(c *request.Context) {
	// error handling
	c.ResponseWriter.Header().Set(headers.CacheControl, errCacheControl)

	query, queryErr := buildQuery(c)
	if queryErr != nil {
		extractQueryError(c, queryErr)
		c.WriteResult()
		return
	}
	if query.Service.Environment == "" {
		query.Service.Environment = h.defaultServiceEnvironment
	}

	// Only service, and not agent, is known for config queries.
	// For anonymous/untrusted agents, we filter the results using
	// query.InsecureAgents below.
	authResource := auth.Resource{ServiceName: query.Service.Name}
	if err := auth.Authorize(c.Request.Context(), auth.ActionAgentConfig, authResource); err != nil {
		if errors.Is(err, auth.ErrUnauthorized) {
			id := request.IDResponseErrorsForbidden
			status := request.MapResultIDToStatus[id]
			c.Result.Set(id, status.Code, err.Error(), nil, nil)
		} else {
			c.Result.SetDefault(request.IDResponseErrorsServiceUnavailable)
			c.Result.Err = err
		}
		c.WriteResult()
		return
	}
	if c.Authentication.Method == auth.MethodAnonymous {
		// Unauthenticated client, restrict results.
		query.InsecureAgents = h.allowAnonymousAgents
	}

	result, err := h.f.Fetch(c.Request.Context(), query)
	if err != nil {
		extractInternalError(c, err)
		c.WriteResult()
		return
	}

	// configuration successfully fetched
	c.ResponseWriter.Header().Set(headers.CacheControl, h.cacheControl)
	c.ResponseWriter.Header().Set(headers.Etag, fmt.Sprintf("\"%s\"", result.Source.Etag))
	c.ResponseWriter.Header().Set(headers.AccessControlExposeHeaders, headers.Etag)

	if result.Source.Etag == ifNoneMatch(c) {
		c.Result.SetDefault(request.IDResponseValidNotModified)
	} else {
		c.Result.SetWithBody(request.IDResponseValidOK, result.Source.Settings)
	}
	c.WriteResult()
}

func buildQuery(c *request.Context) (agentcfg.Query, error) {
	r := c.Request

	var query agentcfg.Query
	switch r.Method {
	case http.MethodPost:
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			return query, err
		}
	case http.MethodGet:
		params := r.URL.Query()
		query = agentcfg.Query{
			Service: agentcfg.Service{
				Name:        params.Get(agentcfg.ServiceName),
				Environment: params.Get(agentcfg.ServiceEnv),
			},
		}
	default:
		if err := errors.Errorf("%s: %s", msgMethodUnsupported, r.Method); err != nil {
			return query, err
		}
	}
	if query.Service.Name == "" {
		return query, errors.New(agentcfg.ServiceName + " is required")
	}

	query.Etag = ifNoneMatch(c)
	return query, nil
}

func extractInternalError(c *request.Context, err error) {
	msg := err.Error()
	var body interface{}
	var keyword string
	switch {
	case strings.Contains(msg, agentcfg.ErrMsgSendToKibanaFailed):
		body = authErrMsg(c, msg, agentcfg.ErrMsgSendToKibanaFailed)
		keyword = agentcfg.ErrMsgSendToKibanaFailed

	case strings.Contains(msg, agentcfg.ErrMsgReadKibanaResponse):
		body = authErrMsg(c, msg, agentcfg.ErrMsgReadKibanaResponse)
		keyword = agentcfg.ErrMsgReadKibanaResponse

	case strings.Contains(msg, agentcfg.ErrUnauthorized):
		fullMsg := "APM Server is not authorized to query Kibana. " +
			"Please configure apm-server.kibana.username and apm-server.kibana.password, " +
			"and ensure the user has the necessary privileges."
		body = authErrMsg(c, fullMsg, agentcfg.ErrUnauthorized)
		keyword = agentcfg.ErrUnauthorized

	case strings.Contains(msg, agentcfg.ErrNoValidElasticsearchConfig):
		body = "Your Elasticsearch configuration does not support agent config queries. Check your configurations at `output.elasticsearch` or `apm-server.agent.config.elasticsearch`."
		c.Result.Set(
			request.IDResponseErrorsForbidden,
			http.StatusForbidden,
			keyword,
			body,
			err,
		)
		return

	case strings.Contains(msg, agentcfg.ErrInfrastructureNotReady):
		body = "Agent configuration infrastructure is not ready. Please retry later."
		c.Result.Set(
			request.IDResponseErrorsServiceUnavailable,
			http.StatusServiceUnavailable,
			keyword,
			body,
			err,
		)
		return

	default:
		body = authErrMsg(c, msg, msgServiceUnavailable)
		keyword = msgServiceUnavailable
	}

	c.Result.Set(request.IDResponseErrorsServiceUnavailable,
		http.StatusServiceUnavailable,
		keyword,
		body,
		err)
}

func extractQueryError(c *request.Context, err error) {
	msg := err.Error()
	if strings.Contains(msg, msgMethodUnsupported) {
		c.Result.Set(request.IDResponseErrorsMethodNotAllowed,
			http.StatusMethodNotAllowed,
			msgMethodUnsupported,
			authErrMsg(c, msg, msgMethodUnsupported),
			err)
		return
	}
	c.Result.Set(request.IDResponseErrorsInvalidQuery,
		http.StatusBadRequest,
		msgInvalidQuery,
		authErrMsg(c, msg, msgInvalidQuery),
		err)
}

func authErrMsg(c *request.Context, fullMsg, shortMsg string) string {
	if c.Authentication.Method != auth.MethodAnonymous {
		return fullMsg
	}
	return shortMsg
}

func ifNoneMatch(c *request.Context) string {
	if h := c.Request.Header.Get(headers.IfNoneMatch); h != "" {
		return strings.Replace(h, "\"", "", -1)
	}
	return c.Request.URL.Query().Get(agentcfg.Etag)
}
