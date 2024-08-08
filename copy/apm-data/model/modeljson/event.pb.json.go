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

package modeljson

import (
	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
)

func EventModelJSON(e *modelpb.Event, out *modeljson.Event) {
	*out = modeljson.Event{
		Outcome:  e.Outcome,
		Action:   e.Action,
		Dataset:  e.Dataset,
		Kind:     e.Kind,
		Category: e.Category,
		Module:   e.Module,
		Type:     e.Type,
		Duration: e.Duration,
		Severity: e.Severity,
	}
	if e.SuccessCount != nil {
		out.SuccessCount = modeljson.SummaryMetric{
			Count: e.SuccessCount.Count,
			Sum:   e.SuccessCount.Sum,
		}
	}
	if e.Received != 0 {
		out.Received = modeljson.Time(modelpb.ToTime(e.Received))
	}
}
