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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestIngestPipeline(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServer(t)

	tracer := srv.Tracer()
	tx := tracer.StartTransaction("name", "type")
	span := tx.StartSpan("name", "type", nil)
	// If a destination address is recorded, and it is a valid
	// IPv4 or IPv6 address, it will be copied to destination.ip.
	span.Context.SetDestinationAddress("::1", 1234)
	span.End()
	tx.End()
	tracer.Flush(nil)

	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*", estest.TermQuery{
		Field: "span.id",
		Value: span.TraceContext().Span.String(),
	})
	require.Len(t, result.Hits.Hits, 1)

	destinationIP := gjson.GetBytes(result.Hits.Hits[0].RawSource, "destination.ip")
	assert.True(t, destinationIP.Exists())
	assert.Equal(t, "::1", destinationIP.String())
}
