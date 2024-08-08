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

type Span struct {
	Message             *Message           `json:"message,omitempty"`
	Composite           *SpanComposite     `json:"composite,omitempty"`
	Destination         *SpanDestination   `json:"destination,omitempty"`
	DB                  *DB                `json:"db,omitempty"`
	Sync                *bool              `json:"sync,omitempty"`
	Kind                string             `json:"kind,omitempty"`
	Action              string             `json:"action,omitempty"`
	Subtype             string             `json:"subtype,omitempty"`
	ID                  string             `json:"id,omitempty"`
	Type                string             `json:"type,omitempty"`
	Name                string             `json:"name,omitempty"`
	Stacktrace          []StacktraceFrame  `json:"stacktrace,omitempty"`
	Links               []SpanLink         `json:"links,omitempty"`
	SelfTime            AggregatedDuration `json:"self_time,omitempty"`
	RepresentativeCount float64            `json:"representative_count,omitempty"`
}

type Code struct {
	Stacktrace string `json:"stacktrace,omitempty"`
}

type SpanDestination struct {
	Service SpanDestinationService `json:"service"`
}

type SpanDestinationService struct {
	Type         string             `json:"type,omitempty"`
	Name         string             `json:"name,omitempty"`
	Resource     string             `json:"resource,omitempty"`
	ResponseTime AggregatedDuration `json:"response_time,omitempty"`
}

type SpanComposite struct {
	CompressionStrategy string           `json:"compression_strategy"`
	Count               int              `json:"count"`
	Sum                 SpanCompositeSum `json:"sum"`
}

type SpanCompositeSum struct {
	US int64 `json:"us"`
}

type DB struct {
	RowsAffected *uint32 `json:"rows_affected,omitempty"`
	Instance     string  `json:"instance,omitempty"`
	Statement    string  `json:"statement,omitempty"`
	Type         string  `json:"type,omitempty"`
	User         DBUser  `json:"user,omitempty"`
	Link         string  `json:"link,omitempty"`
}

type DBUser struct {
	Name string `json:"name,omitempty"`
}

func (d DBUser) isZero() bool {
	return d == DBUser{}
}
