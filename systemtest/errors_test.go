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
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/apm-tools/pkg/approvaltest"
	"github.com/elastic/apm-tools/pkg/espoll"
)

func TestErrorIngest(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

	// TODO(marclop): Update APM go agent to the latest to test this.
	events, err := os.ReadFile("../testdata/intake-v2/errors.ndjson")
	require.NoError(t, err)

	req, _ := http.NewRequest("POST", srv.URL+"/intake/v2/events", bytes.NewReader(events))
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	result := estest.ExpectDocs(t, systemtest.Elasticsearch, "logs-apm.error*", espoll.ExistsQuery{
		Field: "transaction.name",
	})
	approvaltest.ApproveFields(t, t.Name(), result.Hits.Hits)
}

// TestErrorExceptionCause tests hierarchical exception causes,
// which must be obtained from _source due to how the exception
// tree is structured as an array of objects.
func TestErrorExceptionCause(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

	tracer := srv.Tracer()
	tracer.NewError(fmt.Errorf(
		"parent: %w %w",
		fmt.Errorf("child1: %w", errors.New("grandchild")),
		errors.New("child2"),
	)).Send()
	tracer.Flush(nil)

	result := estest.ExpectDocs(t, systemtest.Elasticsearch, "logs-apm.error*", nil)
	errorObj := result.Hits.Hits[0].Source["error"].(map[string]any)
	exceptions := errorObj["exception"].([]any)

	require.Len(t, exceptions, 4)
	assert.Equal(t, "parent: child1: grandchild child2", exceptions[0].(map[string]any)["message"])
	assert.Equal(t, "child1: grandchild", exceptions[1].(map[string]any)["message"])
	assert.Equal(t, "grandchild", exceptions[2].(map[string]any)["message"])
	assert.Equal(t, "child2", exceptions[3].(map[string]any)["message"])
	assert.NotContains(t, exceptions[0], "parent")
	assert.NotContains(t, exceptions[1], "parent")
	assert.NotContains(t, exceptions[2], "parent")
	assert.Equal(t, float64(0), exceptions[3].(map[string]any)["parent"])
}
