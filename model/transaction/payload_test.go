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

package transaction

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"time"

	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/transform"
)

func TestPayloadDecode(t *testing.T) {
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)
	ip := "127.0.0.1"
	for _, test := range []struct {
		input  map[string]interface{}
		err    error
		events []transform.Eventable
	}{
		{input: nil, err: nil},
		{
			input:  map[string]interface{}{},
			err:    nil,
			events: []transform.Eventable{},
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"user":    map[string]interface{}{"ip": ip},
				"transactions": []interface{}{
					map[string]interface{}{
						"id": "45", "type": "transaction",
						"timestamp": timestamp, "duration": 34.9,
					},
				},
			},
			err: nil,
			events: []transform.Eventable{
				&Event{
					Id:        "45",
					Type:      "transaction",
					Timestamp: timestampParsed,
					Duration:  34.9,
					Spans:     []*span.Span{},
				},
			},
		},
	} {
		outputEvents, err := DecodePayload(test.input)
		assert.Equal(t, test.events, outputEvents)
		assert.Equal(t, test.err, err)
	}
}
