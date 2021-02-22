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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestDefaultServiceEnvironment(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.DefaultServiceEnvironment = "default"
	err := srv.Start()
	require.NoError(t, err)

	defer os.Unsetenv("ELASTIC_APM_ENVIRONMENT")
	tracerDefaultEnvironment := srv.Tracer()

	os.Setenv("ELASTIC_APM_ENVIRONMENT", "specified")
	tracerSpecifiedEnvironment := srv.Tracer()

	tracerDefaultEnvironment.StartTransaction("default_environment", "type").End()
	tracerDefaultEnvironment.Flush(nil)

	tracerSpecifiedEnvironment.StartTransaction("specified_environment", "type").End()
	tracerSpecifiedEnvironment.Flush(nil)

	result := systemtest.Elasticsearch.ExpectMinDocs(t, 2, "apm-*",
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	environments := make(map[string]string)
	for _, hit := range result.Hits.Hits {
		transactionName := gjson.GetBytes(hit.RawSource, "transaction.name").String()
		serviceEnvironment := gjson.GetBytes(hit.RawSource, "service.environment").String()
		environments[transactionName] = serviceEnvironment
	}
	assert.Equal(t, map[string]string{
		"default_environment":   "default",
		"specified_environment": "specified",
	}, environments)
}
