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
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/elastic/beats/libbeat/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/tests"
)

func TestDecodeMessaging(t *testing.T) {
	for _, tc := range []struct {
		name    string
		inp     interface{}
		inpErr  error
		message *Messaging
		outpErr error
	}{
		{name: "empty"},
		{name: "error",
			inpErr: errors.New("error foo")},
		{name: "invalid",
			inp: "foo", outpErr: errors.New("invalid type for message")},
		{name: "no queue.name",
			inp:     map[string]interface{}{"message": map[string]interface{}{"topic.name": "foo"}},
			outpErr: errors.New("error fetching field")},
		{name: "no topic.name",
			inp:     map[string]interface{}{"message": map[string]interface{}{"queue.name": "foo"}},
			outpErr: errors.New("error fetching field")},
		{name: "minimal",
			inp: map[string]interface{}{
				"message": map[string]interface{}{
					"queue": map[string]interface{}{"name": "order"},
					"topic": map[string]interface{}{"name": "routeA"}}},
			message: &Messaging{QueueName: "order", TopicName: "routeA"},
		},
		{name: "full",
			inp: map[string]interface{}{
				"message": map[string]interface{}{
					"queue":   map[string]interface{}{"name": "order"},
					"topic":   map[string]interface{}{"name": "routeA"},
					"body":    "user A ordered book B",
					"headers": map[string]interface{}{"internal": "false", "services": []string{"user", "order"}},
					"age":     map[string]interface{}{"ms": json.Number("1577958057123")}}},
			message: &Messaging{
				QueueName: "order", TopicName: "routeA", Body: tests.StringPtr("user A ordered book B"),
				Headers:     http.Header{"Internal": []string{"false"}, "Services": []string{"user", "order"}},
				AgeMicroSec: tests.IntPtr(1577958057123),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := DecodeMessaging(tc.inp, tc.inpErr)
			if tc.inpErr != nil {
				require.Equal(t, tc.inpErr, err)
			} else if tc.outpErr != nil {
				require.Equal(t, tc.outpErr, err)
			} else {
				require.Nil(t, err)
			}
			assert.Equal(t, tc.message, decoded)
		})
	}
}

func TestMessaging_Fields(t *testing.T) {
	var m *Messaging
	require.Nil(t, m.Fields())

	m = &Messaging{TopicName: "foo", QueueName: "orders"}
	outp := common.MapStr{"topic.name": "foo", "queue.name": "orders"}
	require.Equal(t, outp, m.Fields())

	m.Body = tests.StringPtr("order confirmed")
	m.Headers = http.Header{"Internal": []string{"false"}, "Services": []string{"user", "order"}}
	m.AgeMicroSec = tests.IntPtr(1577958057123)
	outp["body"] = "order confirmed"
	outp["headers"] = http.Header{"Internal": []string{"false"}, "Services": []string{"user", "order"}}
	outp["age.ms"] = 1577958057123
	assert.Equal(t, outp, m.Fields())
}
