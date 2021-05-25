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
	"fmt"
	"net/http"
	"strings"

	"github.com/elastic/beats/v7/libbeat/common"
)

// Message holds information about a recorded message, such as the message body and meta information
type Message struct {
	Body      string
	Headers   http.Header
	AgeMillis *int
	QueueName string
	// TODO: Do we want to add all these keys to Message? Or should they be
	// parsed as individual strings and added to the span in
	// processor/otel/consumer.go?
	RoutingKey string
	System     string
	Operation  string
}

// Name returns a Span name for a message following the transaction/span naming schema.
// https://github.com/elastic/apm/blob/master/specs/agents/tracing-instrumentation-messaging.md#naming
func (m *Message) Name() string {
	return fmt.Sprintf("%s %s %s %s",
		m.System, strings.ToUpper(m.Operation), m.direction(), m.QueueName,
	)
}

func (m *Message) direction() string {
	if m.Operation == "send" {
		return "to"
	}
	return "from"
}

// Fields returns a MapStr holding the transformed message information
func (m *Message) Fields() common.MapStr {
	if m == nil {
		return nil
	}
	var fields mapStr
	if m.QueueName != "" {
		fields.set("queue", common.MapStr{"name": m.QueueName})
	}
	if m.AgeMillis != nil {
		fields.set("age", common.MapStr{"ms": *m.AgeMillis})
	}
	fields.maybeSetString("body", m.Body)
	if len(m.Headers) > 0 {
		fields.set("headers", m.Headers)
	}
	if m.RoutingKey != "" {
		fields.set("routing_key", m.RoutingKey)
	}
	if m.System != "" {
		fields.set("system", m.System)
	}
	if m.Operation != "" {
		fields.set("operation", m.Operation)
	}
	return common.MapStr(fields)
}
