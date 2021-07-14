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
	"context"

	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/v7/libbeat/beat"
)

// APMEvent holds the details of an APM event.
//
// Exactly one of the event fields should be non-nil.
type APMEvent struct {
	Transaction   *Transaction
	Span          *Span
	Metricset     *Metricset
	Error         *Error
	ProfileSample *ProfileSample
}

func (e *APMEvent) appendBeatEvent(ctx context.Context, cfg *transform.Config, out []beat.Event) []beat.Event {
	switch {
	case e.Transaction != nil:
		out = append(out, e.Transaction.toBeatEvent(cfg))
	case e.Span != nil:
		out = append(out, e.Span.toBeatEvent(ctx, cfg))
	case e.Metricset != nil:
		out = append(out, e.Metricset.toBeatEvent(cfg))
	case e.Error != nil:
		out = append(out, e.Error.toBeatEvent(ctx, cfg))
	case e.ProfileSample != nil:
		out = append(out, e.ProfileSample.toBeatEvent(cfg))
	}
	return out
}
