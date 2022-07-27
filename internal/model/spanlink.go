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

import "github.com/elastic/elastic-agent-libs/mapstr"

// SpanLink represents a link between two span events,
// possibly (but not necessarily) in different traces.
type SpanLink struct {
	// Span holds information about the linked span event.
	Span Span

	// Trace holds information about the trace of the linked span event.
	Trace Trace
}

func (l *SpanLink) fields() mapstr.M {
	var fields mapStr
	fields.maybeSetMapStr("span", l.Span.fields())
	fields.maybeSetMapStr("trace", l.Trace.fields())
	return mapstr.M(fields)
}
