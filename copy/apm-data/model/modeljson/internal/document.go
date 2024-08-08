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
	"time"

	"go.elastic.co/fastjson"
)

//go:generate go run go.elastic.co/fastjson/cmd/generate-fastjson -f -o marshal_fastjson.go .

const (
	// timestampFormat formats timestamps according to Elasticsearch's
	// strict_date_optional_time date format, which includes a fractional
	// seconds component.
	timestampFormat = "2006-01-02T15:04:05.000Z07:00"
)

// Document is the Elasticsearch JSON document representation of an APM event.
type Document struct {
	Span        *Span        `json:"span,omitempty"`
	Transaction *Transaction `json:"transaction,omitempty"`
	Metricset   *Metricset   `json:"metricset,omitempty"`
	Error       *Error       `json:"error,omitempty"`

	TimestampStruct *Timestamp              `json:"timestamp,omitempty"`
	Labels          map[string]Label        `json:"labels,omitempty"`
	NumericLabels   map[string]NumericLabel `json:"numeric_labels,omitempty"`

	Cloud       *Cloud       `json:"cloud,omitempty"`
	Service     *Service     `json:"service,omitempty"`
	FAAS        *FAAS        `json:"faas,omitempty"`
	Network     *Network     `json:"network,omitempty"`
	Container   *Container   `json:"container,omitempty"`
	User        *User        `json:"user,omitempty"`
	Device      *Device      `json:"device,omitempty"`
	Kubernetes  *Kubernetes  `json:"kubernetes,omitempty"`
	Observer    *Observer    `json:"observer,omitempty"`
	Agent       *Agent       `json:"agent,omitempty"`
	HTTP        *HTTP        `json:"http,omitempty"`
	UserAgent   *UserAgent   `json:"user_agent,omitempty"`
	Parent      *Parent      `json:"parent,omitempty"`
	Trace       *Trace       `json:"trace,omitempty"`
	Host        *Host        `json:"host,omitempty"`
	URL         *URL         `json:"url,omitempty"`
	Log         *Log         `json:"log,omitempty"`
	Source      *Source      `json:"source,omitempty"`
	Client      *Client      `json:"client,omitempty"`
	Child       *Child       `json:"child,omitempty"`
	Destination *Destination `json:"destination,omitempty"`
	Session     *Session     `json:"session,omitempty"`
	Process     *Process     `json:"process,omitempty"`
	Event       *Event       `json:"event,omitempty"`
	Code        *Code        `json:"code,omitempty"`
	System      *System      `json:"system,omitempty"`

	Timestamp  Time        `json:"@timestamp"`
	DataStream *DataStream `json:"data_stream,omitempty"`
	Message    string      `json:"message,omitempty"`
	DocCount   uint64      `json:"_doc_count,omitempty"`
}

type Time time.Time

func (t Time) MarshalFastJSON(w *fastjson.Writer) error {
	w.RawByte('"')
	w.Time(time.Time(t), timestampFormat)
	w.RawByte('"')
	return nil
}

func (t Time) isZero() bool {
	return time.Time(t).IsZero()
}

type DataStream struct {
	Type      string `json:"type,omitempty"`
	Dataset   string `json:"dataset,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}
