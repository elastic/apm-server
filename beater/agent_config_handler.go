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
	"net/http"

	"github.com/elastic/apm-server/convert"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/kibana"

	"github.com/elastic/apm-server/agentcfg"
)

func agentConfigHandler(kbClient *kibana.Client, config *agentConfig, secretToken string) http.Handler {
	fetcher := agentcfg.NewFetcher(kbClient, config.Cache.Expiration)

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		send := wrap(w, r)
		clientEtag := r.Header.Get("If-None-Match")

		query, requestErr := buildQuery(r)
		cfg, upstreamEtag, internalErr := fetcher.Fetch(query, requestErr)

		switch {
		case requestErr != nil:
			send(requestErr.Error(), http.StatusBadRequest)
		case query == agentcfg.Query{}:
			send(nil, http.StatusMethodNotAllowed)
		case internalErr != nil:
			send(internalErr.Error(), http.StatusInternalServerError)
		case len(cfg) == 0:
			send(nil, http.StatusNotFound)
		case clientEtag != "" && clientEtag == upstreamEtag:
			w.Header().Set("Cache-Control", "max-age=0")
			send(nil, http.StatusNotModified)
		case upstreamEtag != "":
			w.Header().Set("Cache-Control", "max-age=0")
			w.Header().Set("Etag", upstreamEtag)
			fallthrough
		default:
			send(cfg, http.StatusOK)
		}
	})
	return authHandler(secretToken, logHandler(handler))
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
		return
	}

	if err == nil && query.Service.Name == "" {
		err = errors.New(agentcfg.ServiceName + " is required")
	}
	return
}

func wrap(w http.ResponseWriter, r *http.Request) func(interface{}, int) {
	return func(body interface{}, code int) {
		if body == nil {
			w.WriteHeader(code)
		} else {
			send(w, r, body, code)
		}
	}
}
