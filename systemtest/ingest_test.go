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
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestIngestPipelinePipeline(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServer(t)

	tracer := srv.Tracer()
	httpRequest := &http.Request{
		URL: &url.URL{},
		Header: http.Header{
			"User-Agent": []string{
				"Mozilla/5.0 (Windows NT 6.1; Win64; x64; rv:47.0) Gecko/20100101 Firefox/47.0",
			},
		},
	}
	tx := tracer.StartTransaction("name", "type")
	tx.Context.SetHTTPRequest(httpRequest)
	span1 := tx.StartSpan("name", "type", nil)
	// If a destination address is recorded, and it is a valid
	// IPv4 or IPv6 address, it will be copied to destination.ip.
	span1.Context.SetDestinationAddress("::1", 1234)
	span1.End()
	span2 := tx.StartSpan("name", "type", nil)
	span2.Context.SetDestinationAddress("testing.invalid", 1234)
	span2.End()
	tx.End()
	tracer.Flush(nil)

	getDoc := func(query estest.TermQuery) estest.SearchHit {
		result := systemtest.Elasticsearch.ExpectDocs(t, "traces-apm*", query)
		require.Len(t, result.Hits.Hits, 1)
		return result.Hits.Hits[0]
	}

	txDoc := getDoc(estest.TermQuery{Field: "processor.event", Value: "transaction"})
	assert.Equal(t, httpRequest.Header.Get("User-Agent"), gjson.GetBytes(txDoc.RawSource, "user_agent.original").String())
	assert.Equal(t, "Firefox", gjson.GetBytes(txDoc.RawSource, "user_agent.name").String())

	span1Doc := getDoc(estest.TermQuery{Field: "span.id", Value: span1.TraceContext().Span.String()})
	destinationIP := gjson.GetBytes(span1Doc.RawSource, "destination.ip")
	assert.True(t, destinationIP.Exists())
	assert.Equal(t, "::1", destinationIP.String())

	span2Doc := getDoc(estest.TermQuery{Field: "span.id", Value: span2.TraceContext().Span.String()})
	destinationIP = gjson.GetBytes(span2Doc.RawSource, "destination.ip")
	assert.False(t, destinationIP.Exists()) // destination.address is not an IP
}
