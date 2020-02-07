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
)

func TestDecodeMessage(t *testing.T) {
	name, body := "order", "user A ordered book B"
	ageMillis := 1577958057123
	for _, tc := range []struct {
		name    string
		inp     interface{}
		inpErr  error
		message *Message
		outpErr error
	}{
		{name: "empty"},
		{name: "error",
			inpErr: errors.New("error foo")},
		{name: "invalid",
			inp: "foo", outpErr: errors.New("invalid type for message")},
		{name: "valid",
			inp: map[string]interface{}{
				"message": map[string]interface{}{
					"queue":   map[string]interface{}{"name": "order"},
					"body":    "user A ordered book B",
					"headers": map[string]interface{}{"internal": "false", "services": []string{"user", "order"}},
					"age":     map[string]interface{}{"ms": json.Number("1577958057123")}}},
			message: &Message{
				QueueName: &name,
				Body:      &body,
				Headers:   http.Header{"Internal": []string{"false"}, "Services": []string{"user", "order"}},
				AgeMillis: &ageMillis,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := DecodeMessage(tc.inp, tc.inpErr)
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
	name, body := "orders", "order confirmed"
	ageMillis := 1577958057123

	var m *Message
	require.Nil(t, m.Fields())

	m = &Message{}
	require.Equal(t, common.MapStr{}, m.Fields())

	m = &Message{
		QueueName: &name,
		Body:      &body,
		Headers:   http.Header{"Internal": []string{"false"}, "Services": []string{"user", "order"}},
		AgeMillis: &ageMillis,
	}
	outp := common.MapStr{
		"queue":   common.MapStr{"name": "orders"},
		"body":    "order confirmed",
		"headers": http.Header{"Internal": []string{"false"}, "Services": []string{"user", "order"}},
		"age":     common.MapStr{"ms": 1577958057123}}
	assert.Equal(t, outp, m.Fields())
}
