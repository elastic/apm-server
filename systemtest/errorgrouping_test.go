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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/v2"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/apm-tools/pkg/espoll"
)

func TestErrorGroupingName(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

	tracer := srv.Tracer()
	tracer.NewError(errors.New("only_exception_message")).Send()
	tracer.NewErrorLog(apm.ErrorLogRecord{Message: "only_log_message"}).Send()
	tracer.NewErrorLog(apm.ErrorLogRecord{Message: "log_message_overrides", Error: errors.New("exception_message_overridden")}).Send()
	tracer.Flush(nil)

	result := estest.ExpectMinDocs(t, systemtest.Elasticsearch, 3, "logs-apm.error-*", espoll.TermQuery{
		Field: "processor.event",
		Value: "error",
	})

	var names []string
	for _, hit := range result.Hits.Hits {
		values := hit.Fields["error.grouping_name"]
		require.Len(t, values, 1)
		names = append(names, values[0].(string))
	}

	assert.ElementsMatch(t, []string{
		"only_exception_message",
		"only_log_message",
		"log_message_overrides",
	}, names)
}
