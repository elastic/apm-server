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

package root

import (
	"errors"
	"net/http"
	"time"

	"github.com/elastic/beats/v7/libbeat/version"
	"github.com/elastic/elastic-agent-libs/mapstr"

	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/request"
)

// HandlerConfig holds configuration for Handler.
type HandlerConfig struct {
	// PublishReady reports whether or not the server is ready for publishing events.
	PublishReady func() bool

	// Version holds the APM Server version.
	Version string
}

// Handler returns error if route does not exist,
// otherwise returns information about the server. The detail level differs for authenticated and anonymous requests.
func Handler(cfg HandlerConfig) request.Handler {
	return func(c *request.Context) {
		// Only allow GET, HEAD requests
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead:
		default:
			c.Result.SetWithError(
				request.IDResponseErrorsMethodNotAllowed,
				// include a verbose error message to alert users about a common misconfiguration
				errors.New("this is the server information endpoint; did you mean to send data to another endpoint instead?"),
			)
			c.WriteResult()
			return
		}

		serverInfo := mapstr.M{
			"build_date": version.BuildTime().Format(time.RFC3339),
			"build_sha":  version.Commit(),
			"version":    cfg.Version,
		}
		if cfg.PublishReady != nil {
			serverInfo["publish_ready"] = cfg.PublishReady()
		}

		c.Result.SetDefault(request.IDResponseValidOK)
		if c.Authentication.Method != auth.MethodAnonymous {
			c.Result.Body = serverInfo
		}
		c.WriteResult()
	}
}
