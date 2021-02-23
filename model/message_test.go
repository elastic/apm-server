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

package model

import (
	"net/http"
	"testing"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/tests"
)

func TestMessaging_Fields(t *testing.T) {
	var m *Message
	require.Nil(t, m.Fields())

	m = &Message{}
	require.Equal(t, common.MapStr{}, m.Fields())

	m = &Message{
		QueueName: "orders",
		Body:      "order confirmed",
		Headers:   http.Header{"Internal": []string{"false"}, "Services": []string{"user", "order"}},
		AgeMillis: tests.IntPtr(1577958057123),
	}
	outp := common.MapStr{
		"queue":   common.MapStr{"name": "orders"},
		"body":    "order confirmed",
		"headers": http.Header{"Internal": []string{"false"}, "Services": []string{"user", "order"}},
		"age":     common.MapStr{"ms": 1577958057123}}
	assert.Equal(t, outp, m.Fields())
}
