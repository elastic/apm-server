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

package beatertest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/publisher"
	agentconfig "github.com/elastic/elastic-agent-libs/config"

	"github.com/elastic/go-docappender/docappendertest"
)

// ElasticsearchOutputConfig returns "output.elasticsearch" configuration
// which will send docs to a mock Elasticsearch server, which in turn sends
// documents to the returned channel.
func ElasticsearchOutputConfig(t testing.TB) (*agentconfig.C, <-chan []byte) {
	out := make(chan []byte, 10) // TODO(axw) buffering is a bandaid
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		// We must send a valid JSON response for the initial
		// Elasticsearch cluster UUID query.
		fmt.Fprintln(w, `{"version":{"number":"1.2.3"}}`)
	})
	mux.HandleFunc("/_bulk", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		docs, response := docappendertest.DecodeBulkRequest(r)
		defer json.NewEncoder(w).Encode(response)
		for _, doc := range docs {
			var buf bytes.Buffer
			if err := json.Indent(&buf, doc, "", "  "); err != nil {
				panic(err)
			}
			select {
			case <-r.Context().Done():
			case out <- buf.Bytes():
			}
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfg := agentconfig.MustNewConfigFrom(map[string]interface{}{
		"output.elasticsearch": map[string]interface{}{
			"enabled":      true,
			"hosts":        []string{srv.URL},
			"flush_bytes":  "1", // no delay
			"max_requests": "1", // only 1 concurrent request, for event ordering
		},
	})
	return cfg, out
}

type nullOutput struct {
	observer outputs.Observer
}

func init() {
	outputs.RegisterType("null", makeNullOutput)
}

func makeNullOutput(
	_ outputs.IndexManager,
	_ beat.Info,
	observer outputs.Observer,
	_ *agentconfig.C,
) (outputs.Group, error) {
	client := nullOutput{observer: observer}
	return outputs.Success(1, 0, client)
}

func (nullOutput) String() string {
	return "null"
}

func (nullOutput) Close() error {
	return nil
}

func (o nullOutput) Publish(_ context.Context, batch publisher.Batch) error {
	events := batch.Events()
	o.observer.NewBatch(len(events))
	o.observer.Acked(len(events))
	batch.ACK()
	return nil
}
