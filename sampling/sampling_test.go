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

package sampling_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/sampling"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

func TestNewDiscardUnsampledReporter(t *testing.T) {
	var reported []transform.Transformable
	reporter := sampling.NewDiscardUnsampledReporter(
		func(ctx context.Context, req publish.PendingReq) error {
			reported = req.Transformables
			return nil
		},
	)

	t1 := &model.Transaction{Sampled: true}
	t2 := &model.Transaction{Sampled: false}
	t3 := &model.Transaction{Sampled: true}
	span := &model.Span{}

	reporter(context.Background(), publish.PendingReq{
		Transformables: []transform.Transformable{t1, t2, t3, span},
	})

	// Note that t3 gets sent to the back of the slice;
	// this reporter is not order-preserving.
	require.Len(t, reported, 3)
	assert.Equal(t, t1, reported[0])
	assert.Equal(t, span, reported[1])
	assert.Equal(t, t3, reported[2])

	expectedMonitoring := monitoring.MakeFlatSnapshot()
	expectedMonitoring.Ints["transactions_dropped"] = 1

	snapshot := monitoring.CollectFlatSnapshot(
		monitoring.GetRegistry("apm-server.sampling"),
		monitoring.Full,
		false, // expvar
	)
	assert.Equal(t, expectedMonitoring, snapshot)
}

func newBool(v bool) *bool {
	return &v
}
