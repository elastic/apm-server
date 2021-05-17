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
	"context"
	"testing"
	"time"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexTemplateKeywords(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServer(t)

	indexTemplateName := "apm-" + srv.Version
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

	keywordFields := make(map[string]interface{})
	mappings := indexTemplate["mappings"].(map[string]interface{})
	getFlattenedKeywords(mappings["properties"].(map[string]interface{}), "", keywordFields)

	// Check that all keyword fields (ECS and non-ECS) have ignore_above set.
	//
	// ignoreAboveExceptions holds the expected "ignore_above" value for keyword
	// fields, for fields which do not have the standard value of 1024.
	var standardIgnoreAbove float64 = 1024
	ignoreAboveExceptions := map[string]float64{"file.drive_letter": 1}
	for field, mapping := range keywordFields {
		mapping := mapping.(map[string]interface{})
		expect, ok := ignoreAboveExceptions[field]
		if !ok {
			expect = standardIgnoreAbove
		}
		assert.Equal(t, expect, mapping["ignore_above"])
	}
}

func getFlattenedKeywords(properties map[string]interface{}, prefix string, out map[string]interface{}) {
	for field, mapping := range properties {
		mapping := mapping.(map[string]interface{})
		if mapping["type"] == "keyword" {
			out[prefix+field] = mapping
			continue
		}
		if properties, ok := mapping["properties"].(map[string]interface{}); ok {
			getFlattenedKeywords(properties, field+".", out)
		}
	}
}
