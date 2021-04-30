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
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.elastic.co/apm"

	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/convert"
)

const (
	errMaxAgeDuration = 5 * time.Minute

	msgInvalidQuery       = "invalid query"
	msgMethodUnsupported  = "method not supported"
	msgNoKibanaConnection = "unable to retrieve connection to Kibana"
	msgServiceUnavailable = "service unavailable"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.DefaultMonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.acm")

	errCacheControl = fmt.Sprintf("max-age=%v, must-revalidate", errMaxAgeDuration.Seconds())

	// rumAgents keywords (new and old)
	rumAgents = []string{"rum-js", "js-base"}
)

type handler struct {
	f fetcher

	cacheControl, defaultServiceEnvironment string
}

func NewHandler(f fetcher, config *config.KibanaAgentConfig, defaultServiceEnvironment string) request.Handler {
	if f == nil {
		panic("fetcher must not be nil")
	}
	cacheControl := fmt.Sprintf("max-age=%v, must-revalidate", config.Cache.Expiration.Seconds())
	h := &handler{
		f:                         f,
		cacheControl:              cacheControl,
		defaultServiceEnvironment: defaultServiceEnvironment,
	}

	return h.Handle
}

type fetcher interface {
	Fetch(context.Context, agentcfg.Query) (agentcfg.Result, error)
}

// Handler implements request.Handler for managing agent central configuration
// requests.
func (h *handler) Handle(c *request.Context) {
	// error handling
	c.Header().Set(headers.CacheControl, errCacheControl)

	ok := c.RateLimiter == nil || c.RateLimiter.Allow()
	if !ok {
		c.Result.SetDefault(request.IDResponseErrorsRateLimit)
		c.Write()
		return
	}

	query, queryErr := buildQuery(c)
	if queryErr != nil {
		extractQueryError(c, queryErr, c.AuthResult.Authorized)
		c.Write()
		return
	}
	if query.Service.Environment == "" {
		query.Service.Environment = h.defaultServiceEnvironment
	}

	result, err := h.f.Fetch(c.Request.Context(), query)
	if err != nil {
		var verr *agentcfg.ValidationError
		if errors.As(err, &verr) {
			body := verr.Body()
			if strings.HasPrefix(body, agentcfg.ErrMsgKibanaVersionNotCompatible) {
				body = authErrMsg(body, agentcfg.ErrMsgKibanaVersionNotCompatible, c.AuthResult.Authorized)
			}
			c.Result.Set(
				request.IDResponseErrorsServiceUnavailable,
				http.StatusServiceUnavailable,
				verr.Keyword(),
				body,
				verr,
			)
		} else {
			apm.CaptureError(c.Request.Context(), err).Send()
			extractInternalError(c, err, c.AuthResult.Authorized)
		}
		c.Write()
		return
	}

	// configuration successfully fetched
	c.Header().Set(headers.CacheControl, h.cacheControl)
	c.Header().Set(headers.Etag, fmt.Sprintf("\"%s\"", result.Source.Etag))
	c.Header().Set(headers.AccessControlExposeHeaders, headers.Etag)

	if result.Source.Etag == ifNoneMatch(c) {
		c.Result.SetDefault(request.IDResponseValidNotModified)
	} else {
		c.Result.SetWithBody(request.IDResponseValidOK, result.Source.Settings)
	}
	c.Write()
}

func buildQuery(c *request.Context) (query agentcfg.Query, err error) {
	r := c.Request

	switch r.Method {
	case http.MethodPost:
		err = convert.FromReader(r.Body, &query)
	case http.MethodGet:
		params := r.URL.Query()
		query = agentcfg.Query{
			Service: agentcfg.Service{
				Name:        params.Get(agentcfg.ServiceName),
				Environment: params.Get(agentcfg.ServiceEnv),
			},
		}
	default:
		err = errors.Errorf("%s: %s", msgMethodUnsupported, r.Method)
	}

	if err == nil && query.Service.Name == "" {
		err = errors.New(agentcfg.ServiceName + " is required")
	}
	if c.IsRum {
		query.InsecureAgents = rumAgents
	}
	query.Etag = ifNoneMatch(c)
	return
}

func extractInternalError(c *request.Context, err error, withAuth bool) {
	msg := err.Error()
	var body interface{}
	var keyword string
	switch {
	case strings.Contains(msg, agentcfg.ErrMsgSendToKibanaFailed):
		body = authErrMsg(msg, agentcfg.ErrMsgSendToKibanaFailed, withAuth)
		keyword = agentcfg.ErrMsgSendToKibanaFailed

	case strings.Contains(msg, agentcfg.ErrMsgReadKibanaResponse):
		body = authErrMsg(msg, agentcfg.ErrMsgReadKibanaResponse, withAuth)
		keyword = agentcfg.ErrMsgReadKibanaResponse

	case strings.Contains(msg, agentcfg.ErrUnauthorized):
		fullMsg := "APM Server is not authorized to query Kibana. " +
			"Please configure apm-server.kibana.username and apm-server.kibana.password, " +
			"and ensure the user has the necessary privileges."
		body = authErrMsg(fullMsg, agentcfg.ErrUnauthorized, withAuth)
		keyword = agentcfg.ErrUnauthorized

	default:
		body = authErrMsg(msg, msgServiceUnavailable, withAuth)
		keyword = msgServiceUnavailable
	}

	c.Result.Set(request.IDResponseErrorsServiceUnavailable,
		http.StatusServiceUnavailable,
		keyword,
		body,
		err)
}

func extractQueryError(c *request.Context, err error, withAuth bool) {
	msg := err.Error()
	if strings.Contains(msg, msgMethodUnsupported) {
		c.Result.Set(request.IDResponseErrorsMethodNotAllowed,
			http.StatusMethodNotAllowed,
			msgMethodUnsupported,
			authErrMsg(msg, msgMethodUnsupported, withAuth),
			err)
		return
	}
	c.Result.Set(request.IDResponseErrorsInvalidQuery,
		http.StatusBadRequest,
		msgInvalidQuery,
		authErrMsg(msg, msgInvalidQuery, withAuth),
		err)
}

func authErrMsg(fullMsg, shortMsg string, withAuth bool) string {
	if withAuth {
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
