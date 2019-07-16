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
	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/kibana"
)

const (
	headerIfNoneMatch  = "If-None-Match"
	headerEtag         = "Etag"
	headerCacheControl = "Cache-Control"

	errMaxAgeDuration = 5 * time.Minute

	errMsgConfigNotFound             = "no configuration available"
	errMsgInvalidQuery               = "invalid query"
	errMsgKibanaDisabled             = "disabled Kibana configuration"
	errMsgKibanaVersionNotCompatible = "not a compatible Kibana version"
	errMsgMethodUnsupported          = "method not supported"
	errMsgNoKibanaConnection         = "unable to retrieve connection to Kibana"
	errMsgServiceUnavailable         = "service unavailable"
)

var (
	minKibanaVersion = common.MustNewVersion("7.3.0")
	errCacheControl  = fmt.Sprintf("max-age=%v, must-revalidate", errMaxAgeDuration.Seconds())
)

func agentConfigHandler(kbClient kibana.Client, config *agentConfig, secretToken string) http.Handler {
	cacheControl := fmt.Sprintf("max-age=%v, must-revalidate", config.Cache.Expiration.Seconds())
	fetcher := agentcfg.NewFetcher(kbClient, config.Cache.Expiration)

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendResp := wrap(w, r)
		sendErr := wrapErr(w, r, secretToken)

		if valid, shortMsg, detailMsg := validateKbClient(kbClient); !valid {
			sendErr(http.StatusServiceUnavailable, shortMsg, detailMsg)
			return
		}

		query, requestErr := buildQuery(r)
		if requestErr != nil {
			if strings.Contains(requestErr.Error(), errMsgMethodUnsupported) {
				sendErr(http.StatusMethodNotAllowed, errMsgMethodUnsupported, requestErr.Error())
				return
			}
			sendErr(http.StatusBadRequest, errMsgInvalidQuery, requestErr.Error())
			return
		}

		cfg, upstreamEtag, internalErr := fetcher.Fetch(query, nil)
		if internalErr != nil {
			sendErr(http.StatusServiceUnavailable, internalErrMsg(internalErr.Error()), internalErr.Error())
			return
		}

		etag := fmt.Sprintf("\"%s\"", upstreamEtag)
		switch {
		case len(cfg) == 0:
			sendResp(nil, http.StatusOK, cacheControl)
		case upstreamEtag == "":
			sendResp(cfg, http.StatusOK, cacheControl)
		case etag == r.Header.Get(headerIfNoneMatch):
			w.Header().Set(headerEtag, etag)
			sendResp(nil, http.StatusNotModified, cacheControl)
		default:
			w.Header().Set(headerEtag, etag)
			sendResp(cfg, http.StatusOK, cacheControl)
		}
	})

	return logHandler(
		killSwitchHandler(kbClient != nil,
			authHandler(secretToken, handler)))
}

func validateKbClient(client kibana.Client) (bool, string, string) {
	if client == nil {
		return false, errMsgKibanaDisabled, errMsgKibanaDisabled
	}
	if !client.Connected() {
		return false, errMsgNoKibanaConnection, errMsgNoKibanaConnection
	}
	if supported, _ := client.SupportsVersion(minKibanaVersion); !supported {
		version, _ := client.GetVersion()
		return false, errMsgKibanaVersionNotCompatible, fmt.Sprintf("min required Kibana version %+v, "+
			"configured Kibana version %+v", minKibanaVersion, version)
	}
	return true, "", ""
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

func wrap(w http.ResponseWriter, r *http.Request) func(interface{}, int, string) {
	return func(body interface{}, code int, cacheControl string) {
		w.Header().Set(headerCacheControl, cacheControl)
		if body == nil {
			w.WriteHeader(code)
			return
		}
		send(w, r, body, code)
	}
}

func wrapErr(w http.ResponseWriter, r *http.Request, token string) func(int, string, string) {
	authErrMsg := func(errMsg, logMsg string) map[string]string {
		if token == "" {
			return map[string]string{"error": errMsg}
		}
		return map[string]string{"error": logMsg}
	}

	return func(status int, errMsg, logMsg string) {
		requestLogger(r).Errorw("error handling request",
			"response_code", status, "error", logMsg)

		w.Header().Set(headerCacheControl, errCacheControl)
		body := authErrMsg(errMsg, logMsg)
		send(w, r, body, status)
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
	return errMsgServiceUnavailable
}
