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

package beater

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/beats/libbeat/common"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/kibana"
)

const (
	errMsgMethodUnsupported          = "method not supported"
	errMaxAgeDuration                = 5 * time.Minute
	errMsgConfigNotFound             = "no config found for"
	errMsgKibanaVersionNotCompatible = "not a compatible Kibana version"
	errMsgNoKibanaConnection         = "unable to retrieve connection to Kibana"
	errMsgInvalidQuery               = "invalid query"
	errMsgKibanaDisabled             = "kibana config disabled"
)

var (
	minKibanaVersion = common.MustNewVersion("7.3.0")
)

func agentConfigHandler(kbClient kibana.Client, config *agentConfig, secretToken string) http.Handler {

	authErrMsg := func(errMsg, logMsg string) string {
		if secretToken == "" {
			return errMsg
		}
		return logMsg
	}

	fetcher := agentcfg.NewFetcher(kbClient, config.Cache.Expiration)
	defaultHeaderCacheControl := fmt.Sprintf("max-age=%v, must-revalidate", config.Cache.Expiration.Seconds())
	errHeaderCacheControl := fmt.Sprintf("max-age=%v, must-revalidate", errMaxAgeDuration.Seconds())

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendResp := wrap(w, r)

		if kbClient == nil {
			sendResp(errMsgKibanaDisabled, http.StatusServiceUnavailable, errHeaderCacheControl, errMsgKibanaDisabled)
			return
		}
		if !kbClient.Connected() {
			sendResp(errMsgNoKibanaConnection, http.StatusServiceUnavailable, errHeaderCacheControl, errMsgNoKibanaConnection)
			return
		}
		if supported, _ := kbClient.SupportsVersion(minKibanaVersion); !supported {
			version, _ := kbClient.GetVersion()
			logMsg := fmt.Sprintf("min required Kibana version %+v, configured Kibana version %+v",
				minKibanaVersion, version)
			sendResp(authErrMsg(errMsgKibanaVersionNotCompatible, logMsg), http.StatusServiceUnavailable,
				errHeaderCacheControl, logMsg)
			return
		}

		query, requestErr := buildQuery(r)
		if requestErr != nil {
			if strings.Contains(requestErr.Error(), errMsgMethodUnsupported) {
				sendResp(authErrMsg(errMsgMethodUnsupported, requestErr.Error()), http.StatusMethodNotAllowed, errHeaderCacheControl, requestErr.Error())
				return
			}
			sendResp(authErrMsg(errMsgInvalidQuery, requestErr.Error()), http.StatusBadRequest, errHeaderCacheControl, requestErr.Error())
			return
		}

		cfg, upstreamEtag, internalErr := fetcher.Fetch(query, nil)
		etag := fmt.Sprintf("\"%s\"", upstreamEtag)

		switch {
		case internalErr != nil:
			logMsg := internalErr.Error()
			sendResp(authErrMsg(internalErrMsg(logMsg), logMsg), http.StatusServiceUnavailable,
				errHeaderCacheControl, logMsg)
		case len(cfg) == 0:
			logMsg := fmt.Sprintf("%s %s", errMsgConfigNotFound, query.ID())
			sendResp(authErrMsg(errMsgConfigNotFound, logMsg), http.StatusNotFound, errHeaderCacheControl, logMsg)
		case upstreamEtag == "":
			sendResp(cfg, http.StatusOK, defaultHeaderCacheControl, "")
		case etag == r.Header.Get(headers.IfNoneMatch):
			w.Header().Set(headers.Etag, etag)
			sendResp(nil, http.StatusNotModified, defaultHeaderCacheControl, "")
		default:
			w.Header().Set(headers.Etag, etag)
			sendResp(cfg, http.StatusOK, defaultHeaderCacheControl, "")
		}
	})

	return logHandler(
		killSwitchHandler(kbClient != nil,
			authHandler(secretToken, handler)))
}

// Returns (zero, error) if request body can't be unmarshalled or service.name is missing
// Returns (zero, zero) if request method is not GET or POST
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
		err = errors.Errorf("%s: %s", errMsgMethodUnsupported, r.Method)
	}

	if err == nil && query.Service.Name == "" {
		err = errors.New(agentcfg.ServiceName + " is required")
	}
	return
}

func wrap(w http.ResponseWriter, r *http.Request) func(interface{}, int, string, string) {
	return func(body interface{}, code int, cacheControl string, errMsg string) {
		w.Header().Set(headers.CacheControl, cacheControl)

		if code >= http.StatusBadRequest {
			requestLogger(r).Errorw("error handling request", "response_code", code,
				"body", body, "error", errMsg)
		}

		if body == nil {
			w.WriteHeader(code)
			return
		}
		send(w, r, body, code)
	}
}

func internalErrMsg(msg string) string {
	switch {
	case strings.Contains(msg, agentcfg.ErrMsgSendToKibanaFailed):
		return agentcfg.ErrMsgSendToKibanaFailed
	case strings.Contains(msg, agentcfg.ErrMsgMultipleChoices):
		return agentcfg.ErrMsgMultipleChoices
	case strings.Contains(msg, agentcfg.ErrMsgReadKibanaResponse):
		return agentcfg.ErrMsgReadKibanaResponse
	}
	return ""
}
