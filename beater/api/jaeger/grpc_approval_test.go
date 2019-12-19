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

package jaeger

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"

	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests/approvals"
)

// testdata are created via a modified version of the hotrod example
// https://github.com/jaegertracing/jaeger/tree/master/examples/hotrod
func TestGRPCSpansToEvents(t *testing.T) {
	for _, name := range []string{
		"batch_1", "batch_2",
	} {
		t.Run(name, func(t *testing.T) {
			tc := testCollector{}
			f := filepath.Join("testdata", name)
			data, err := ioutil.ReadFile(f + ".json")
			require.NoError(t, err)
			var request api_v2.PostSpansRequest
			require.NoError(t, json.Unmarshal(data, &request))
			tc.request = &request
			var events []beat.Event
			tc.reporter = func(ctx context.Context, req publish.PendingReq) error {
				for _, transformable := range req.Transformables {
					events = append(events, transformable.Transform(req.Tcontext)...)
				}
				require.NoError(t, approvals.ApproveEvents(events, f, ""))
				return nil
			}
			tc.setup(t)

			_, err = tc.collector.PostSpans(context.Background(), tc.request)
			require.NoError(t, err)
		})
	}
}
