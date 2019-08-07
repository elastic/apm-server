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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/beats/libbeat/common"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/kibana"
)

const (
	errMaxAgeDuration = 5 * time.Minute

	msgInvalidQuery               = "invalid query"
	msgKibanaDisabled             = "disabled Kibana configuration"
	msgKibanaVersionNotCompatible = "not a compatible Kibana version"
	msgMethodUnsupported          = "method not supported"
	msgNoKibanaConnection         = "unable to retrieve connection to Kibana"
	msgServiceUnavailable         = "service unavailable"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.MonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.acm", monitoring.PublishExpvar)

	errMsgKibanaDisabled     = errors.New(msgKibanaDisabled)
	errMsgNoKibanaConnection = errors.New(msgNoKibanaConnection)

	minKibanaVersion = common.MustNewVersion("7.3.0")
	errCacheControl  = fmt.Sprintf("max-age=%v, must-revalidate", errMaxAgeDuration.Seconds())
)

// Handler returns a request.Handler for managing agent central configuration requests.
func Handler(kbClient kibana.Client, config *config.AgentConfig) request.Handler {
	cacheControl := fmt.Sprintf("max-age=%v, must-revalidate", config.Cache.Expiration.Seconds())
	fetcher := agentcfg.NewFetcher(kbClient, config.Cache.Expiration)

	return func(c *request.Context) {
		// error handling
		c.Header().Set(headers.CacheControl, errCacheControl)

		if valid := validateKbClient(c, kbClient, c.TokenSet); !valid {
			c.Write()
			return
		}

		query, queryErr := buildQuery(c.Request)
		if queryErr != nil {
			extractQueryError(c, queryErr, c.TokenSet)
			c.Write()
			return
		}

		cfg, upstreamEtag, err := fetcher.Fetch(query, nil)
		if err != nil {
			extractInternalError(c, err, c.TokenSet)
			c.Write()
			return
		}

		// configuration successfully fetched
		c.Header().Set(headers.CacheControl, cacheControl)
		etag := fmt.Sprintf("\"%s\"", upstreamEtag)
		c.Header().Set(headers.Etag, etag)
		if etag == c.Request.Header.Get(headers.IfNoneMatch) {
			c.Result.SetDefault(request.IDResponseValidNotModified)
		} else {
			c.Result.SetWithBody(request.IDResponseValidOK, cfg)
		}
		c.Write()
	}
}

func validateKbClient(c *request.Context, client kibana.Client, withAuth bool) bool {
	if client == nil {
		c.Result.Set(request.IDResponseErrorsServiceUnavailable,
			http.StatusServiceUnavailable,
			msgKibanaDisabled,
			msgKibanaDisabled,
			errMsgKibanaDisabled)
		return false
	}
	if !client.Connected() {
		c.Result.Set(request.IDResponseErrorsServiceUnavailable,
			http.StatusServiceUnavailable,
			msgNoKibanaConnection,
			msgNoKibanaConnection,
			errMsgNoKibanaConnection)
		return false
	}
	if supported, _ := client.SupportsVersion(minKibanaVersion); !supported {
		version, _ := client.GetVersion()

		errMsg := fmt.Sprintf("%s: min version %+v, configured version %+v",
			msgKibanaVersionNotCompatible, minKibanaVersion, version.String())
		body := authErrMsg(errMsg, msgKibanaVersionNotCompatible, withAuth)
		c.Result.Set(request.IDResponseErrorsServiceUnavailable,
			http.StatusServiceUnavailable,
			msgKibanaVersionNotCompatible,
			body,
			errors.New(errMsg))
		return false
	}
	return true
}
func buildQuery(r *http.Request) (query agentcfg.Query, err error) {
	switch r.Method {
	case http.MethodPost:
		err = convert.FromReader(r.Body, &query)
	case http.MethodGet:
		params := r.URL.Query()
		query = agentcfg.NewQuery(
			params.Get(agentcfg.ServiceName),
			params.Get(agentcfg.ServiceEnv),
		)
	default:
		err = errors.Errorf("%s: %s", msgMethodUnsupported, r.Method)
	}

	if err == nil && query.Service.Name == "" {
		err = errors.New(agentcfg.ServiceName + " is required")
	}
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

	case strings.Contains(msg, agentcfg.ErrMsgMultipleChoices):
		body = authErrMsg(msg, agentcfg.ErrMsgMultipleChoices, withAuth)
		keyword = agentcfg.ErrMsgMultipleChoices

	case strings.Contains(msg, agentcfg.ErrMsgReadKibanaResponse):
		body = authErrMsg(msg, agentcfg.ErrMsgReadKibanaResponse, withAuth)
		keyword = agentcfg.ErrMsgReadKibanaResponse

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
