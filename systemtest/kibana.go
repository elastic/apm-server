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

package systemtest

import (
	"log"
	"net/url"

	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/fleettest"
)

const (
	adminKibanaUser = adminElasticsearchUser
	adminKibanaPass = adminElasticsearchPass
)

var (
	// KibanaURL is the base URL for Kibana, including userinfo for
	// authenticating as the admin user.
	KibanaURL *url.URL

	// Fleet is a Fleet API client for use in tests.
	Fleet *fleettest.Client
)

func init() {
	kibanaConfig := apmservertest.DefaultConfig().Kibana
	u, err := url.Parse(kibanaConfig.Host)
	if err != nil {
		log.Fatal(err)
	}
	u.User = url.UserPassword(adminKibanaUser, adminKibanaPass)
	KibanaURL = u
	Fleet = fleettest.NewClient(KibanaURL.String())
}
