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
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

func TestIndexTemplateCoverage(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServer(t)

	// Index each supported event type.
	var totalEvents int
	for _, payloadFile := range []string{
		"../testdata/intake-v2/errors.ndjson",
		"../testdata/intake-v2/metricsets.ndjson",
		"../testdata/intake-v2/spans.ndjson",
		"../testdata/intake-v2/transactions.ndjson",
	} {
		data, err := ioutil.ReadFile(payloadFile)
		require.NoError(t, err)
		req, _ := http.NewRequest("POST", srv.URL+"/intake/v2/events?verbose=true", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/x-ndjson")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var result struct {
			Accepted int
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.NotZero(t, result.Accepted)
		totalEvents += result.Accepted
	}

	// Wait for events to be indexed.
	systemtest.Elasticsearch.ExpectMinDocs(t, totalEvents, "apm-*", estest.BoolQuery{
		MustNot: []interface{}{estest.TermQuery{Field: "processor.event", Value: "onboarding"}},
	})

	// Check index mappings are covered by the template with the exception of known dynamic fields (e.g. labels).
	var indexMappings map[string]struct {
		Mappings map[string]interface{}
	}
	_, err := systemtest.Elasticsearch.Do(context.Background(),
		&esapi.IndicesGetMappingRequest{Index: []string{"apm-*"}},
		&indexMappings,
	)
	require.NoError(t, err)

	indexTemplate := getIndexTemplate(t, srv.Version)
	indexTemplateFlattenedFields := make(map[string]interface{})
	indexTemplateMappings := indexTemplate["mappings"].(map[string]interface{})
	getFlattenedFields(indexTemplateMappings["properties"].(map[string]interface{}), "", indexTemplateFlattenedFields)

	knownMetrics := []string{
		"negative", // negative.d.o.t.t.e.d
		"dotted",   // dotted.float.gauge
		"go",       // go.memstats.heap.sys
		"short_gauge",
		"integer_gauge",
		"long_gauge",
		"float_gauge",
		"double_gauge",
		"byte_counter",
		"short_counter",
		"latency_distribution",
	}

	for index, indexMappings := range indexMappings {
		metricIndex := strings.Contains(index, "-metric-")
		indexFlattenedFields := make(map[string]interface{})
		getFlattenedFields(indexMappings.Mappings["properties"].(map[string]interface{}), "", indexFlattenedFields)
		for field := range indexFlattenedFields {
			if strings.HasPrefix(field, "labels.") || strings.HasPrefix(field, "transaction.marks.") {
				// Labels and RUM page marks are dynamically indexed.
				continue
			}
			_, ok := indexTemplateFlattenedFields[field]
			if !ok && metricIndex {
				var isKnownMetric bool
				for _, knownMetric := range knownMetrics {
					if strings.HasPrefix(field, knownMetric) {
						isKnownMetric = true
						break
					}
				}
				if isKnownMetric {
					continue
				}
			}
			assert.True(t, ok, "%s: field %s not defined in index template", index, field)
		}
	}
}

func getIndexTemplate(t testing.TB, serverVersion string) map[string]interface{} {
	indexTemplateName := "apm-" + serverVersion
	indexTemplates := make(map[string]interface{})

	// Wait for the index template to be created.
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case <-timeout.C:
			t.Fatal("timed out waiting for index template")
		default:
		}
		if _, err := systemtest.Elasticsearch.Do(context.Background(),
			&esapi.IndicesGetTemplateRequest{Name: []string{indexTemplateName}},
			&indexTemplates,
		); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	require.Len(t, indexTemplates, 1)
	require.Contains(t, indexTemplates, indexTemplateName)
	indexTemplate := indexTemplates[indexTemplateName].(map[string]interface{})
	return indexTemplate
}

func getFlattenedFields(properties map[string]interface{}, prefix string, out map[string]interface{}) {
	for field, mapping := range properties {
		mapping := mapping.(map[string]interface{})
		out[prefix+field] = mapping
		if properties, ok := mapping["properties"].(map[string]interface{}); ok {
			getFlattenedFields(properties, prefix+field+".", out)
		}
	}
}
