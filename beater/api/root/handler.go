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
	"time"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
	"github.com/elastic/beats/libbeat/version"

	"github.com/elastic/apm-server/authorization"
	"github.com/elastic/apm-server/beater/request"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.MonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.root", monitoring.PublishExpvar)
)

// Handler returns error if route does not exist,
// otherwise returns information about the server. The detail level differs for authorized and non-authorized requests.
//TODO: only allow GET, HEAD requests (breaking change)
//TODO: require authorization if configured (breaking change)
func Handler() request.Handler {
	serverInfo := common.MapStr{
		"build_date": version.BuildTime().Format(time.RFC3339),
		"build_sha":  version.Commit(),
		"version":    version.GetDefaultVersion(),
	}

	return func(c *request.Context) {
		if c.Request.URL.Path != "/" {
			c.Result.SetDefault(request.IDResponseErrorsNotFound)
			c.Write()
			return
		}

		c.Result.SetDefault(request.IDResponseValidOK)
		authorized, err := c.Authorization.AuthorizedFor("", authorization.PrivilegeAccess)
		if err != nil {
			c.Result.Err = err
		}
		if authorized {
			c.Result.Body = serverInfo
		}

		c.Write()
	}
}
