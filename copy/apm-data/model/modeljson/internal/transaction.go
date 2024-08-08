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

type Transaction struct {
	SpanCount             SpanCount                     `json:"span_count,omitempty"`
	UserExperience        *UserExperience               `json:"experience,omitempty"`
	Custom                KeyValueSlice                 `json:"custom,omitempty"`
	Marks                 map[string]map[string]float64 `json:"marks,omitempty"`
	Message               *Message                      `json:"message,omitempty"`
	Type                  string                        `json:"type,omitempty"`
	Name                  string                        `json:"name,omitempty"`
	Result                string                        `json:"result,omitempty"`
	ID                    string                        `json:"id,omitempty"`
	DurationHistogram     Histogram                     `json:"duration.histogram,omitempty"`
	DroppedSpansStats     []DroppedSpanStats            `json:"dropped_spans_stats,omitempty"`
	ProfilerStackTraceIds []string                      `json:"profiler_stack_trace_ids,omitempty"`
	DurationSummary       SummaryMetric                 `json:"duration.summary,omitempty"`
	RepresentativeCount   float64                       `json:"representative_count,omitempty"`
	Sampled               bool                          `json:"sampled,omitempty"`
	Root                  bool                          `json:"root,omitempty"`
}

type SpanCount struct {
	Dropped *uint32 `json:"dropped,omitempty"`
	Started *uint32 `json:"started,omitempty"`
}

func (s *SpanCount) isZero() bool {
	return s.Dropped == nil && s.Started == nil
}

type DroppedSpanStats struct {
	DestinationServiceResource string             `json:"destination_service_resource,omitempty"`
	ServiceTargetType          string             `json:"service_target_type,omitempty"`
	ServiceTargetName          string             `json:"service_target_name,omitempty"`
	Outcome                    string             `json:"outcome,omitempty"`
	Duration                   AggregatedDuration `json:"duration,omitempty"`
}
