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
	"errors"
	"net/http"

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/utility"
)

// Message holds information about a recorded message, such as the message body and meta information
type Message struct {
	Body        *string
	Headers     http.Header
	AgeMicroSec *int
	Operation   *string
	QueueName   *string
}

// DecodeMessage parses a Message from given input
func DecodeMessage(input interface{}, err error) (*Message, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for message")
	}
	decoder := utility.ManualDecoder{}

	messageInp := decoder.MapStr(raw, "message")
	if decoder.Err != nil || messageInp == nil {
		return nil, decoder.Err
	}
	m := Message{
		QueueName:   decoder.StringPtr(messageInp, "name", "queue"),
		Body:        decoder.StringPtr(messageInp, "body"),
		Headers:     decoder.Headers(messageInp),
		AgeMicroSec: decoder.IntPtr(messageInp, "ms", "age"),
	}
	if decoder.Err != nil {
		return nil, decoder.Err
	}
	return &m, nil
}

// Fields returns a MapStr holding the transformed message information
func (m *Message) Fields() common.MapStr {
	if m == nil {
		return nil
	}
	fields := common.MapStr{}
	utility.Set(fields, "queue.name", m.QueueName)
	utility.Set(fields, "body", m.Body)
	utility.Set(fields, "headers", m.Headers)
	utility.Set(fields, "age.ms", m.AgeMicroSec)
	utility.Set(fields, "operation", m.Operation)
	return fields
}
