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
	"time"

	"github.com/elastic/apm-server/beater/internal"

	"github.com/elastic/apm-server/server"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/version"
)

func rootHandler(secretToken string) http.Handler {
	serverInfo := common.MapStr{
		"build_date": version.BuildTime().Format(time.RFC3339),
		"build_sha":  version.Commit(),
		"version":    version.GetDefaultVersion(),
	}

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		if isAuthorized(r, secretToken) {
			internal.SendCnt(w, r, server.Result{StatusCode: http.StatusOK, ResponseBody: serverInfo})
			return
		}
		internal.SendCnt(w, r, server.Result{StatusCode: http.StatusOK})
	})
	return logHandler(handler)
}
