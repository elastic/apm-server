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

package systemtest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
)

// withDataStreams runs two sub-tests, calling f with and without data streams enabled.
func withDataStreams(t *testing.T, f func(t *testing.T, unstartedServer *apmservertest.Server)) {
	t.Run("data_streams_disabled", func(t *testing.T) {
		systemtest.CleanupElasticsearch(t)
		srv := apmservertest.NewUnstartedServer(t)
		f(t, srv)
	})
	t.Run("data_streams_enabled", func(t *testing.T) {
		systemtest.CleanupElasticsearch(t)
		err := systemtest.Fleet.InstallPackage(systemtest.IntegrationPackage.Name, systemtest.IntegrationPackage.Version)
		require.NoError(t, err)
		srv := apmservertest.NewUnstartedServer(t)
		srv.Config.DataStreams = &apmservertest.DataStreamsConfig{Enabled: true}
		f(t, srv)
		err = systemtest.Fleet.DeletePackage(systemtest.IntegrationPackage.Name, systemtest.IntegrationPackage.Version)
		require.NoError(t, err)
	})
}
