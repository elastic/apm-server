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

package beater

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

func TestOnboarding(t *testing.T) {
	events := make(chan beat.Event, 1)
	beater, teardown, err := setupServer(t, nil, nil, events)
	require.NoError(t, err)
	defer teardown()

	select {
	case event := <-events:
		listening := event.Fields["observer"].(common.MapStr)["listening"]
		assert.NotEqual(t, "localhost:0", listening)
		assert.Equal(t, beater.config.Host, listening)
		processor := event.Fields["processor"].(common.MapStr)
		assert.Equal(t, "onboarding", processor["name"])
		assert.Equal(t, "onboarding", processor["event"])
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for onboarding event")
	}
}
