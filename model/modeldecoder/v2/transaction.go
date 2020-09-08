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

package v2

import (
	"github.com/elastic/apm-server/model/modeldecoder/nullable"
	"github.com/elastic/beats/v7/libbeat/common"
)

var (
	enumOutcome = []string{"success", "failure", "unknown"}
)

type transactionRoot struct {
	Transaction transaction `json:"transaction" validate:"required"`
}

type age struct {
	Milliseconds nullable.Int `json:"ms"`
}

type context struct {
	Custom   common.MapStr `json:"custom" validate:"patternKeys=regexpNoDotAsteriskQuote"`
	Message  message       `json:"message"`
	Page     page          `json:"page"`
	Response response      `json:"response"`
	Request  request       `json:"request"`
	Service  service       `json:"service"`
	Tags     common.MapStr `json:"tags" validate:"patternKeys=regexpNoDotAsteriskQuote,typesVals=string;bool;number,maxVals=1024"`
	User     user          `json:"user"`
}

type message struct {
	Body    nullable.String     `json:"body"`
	Headers nullable.HTTPHeader `json:"headers"`
	Age     age                 `json:"age"`
	Queue   queue               `json:"queue"`
}

type page struct {
	URL     nullable.String `json:"url"`
	Referer nullable.String `json:"referer"`
}

type queue struct {
	Name nullable.String `json:"name" validate:"max=1024"`
}

type request struct {
	Cookies     nullable.Interface  `json:"cookies"`
	Body        nullable.Interface  `json:"body" validate:"types=string;map[string]interface"`
	Env         nullable.Interface  `json:"env"`
	Headers     nullable.HTTPHeader `json:"headers"`
	HTTPVersion nullable.String     `json:"http_version" validate:"max=1024"`
	Method      nullable.String     `json:"method" validate:"required,max=1024"`
	Socket      socket              `json:"socket"`
	URL         url                 `json:"url"` //TODO(simitt): check validate:"required"`
}

type response struct {
	DecodedBodySize nullable.Float64    `json:"decoded_body_size"`
	EncodedBodySize nullable.Float64    `json:"encoded_body_size"`
	Finished        nullable.Bool       `json:"finished"`
	Headers         nullable.HTTPHeader `json:"headers"`
	HeadersSent     nullable.Bool       `json:"headers_sent"`
	StatusCode      nullable.Int        `json:"status_code"`
	TransferSize    nullable.Float64    `json:"transfer_size"`
}

type service struct {
	Agent       serviceAgent     `json:"agent"`
	Environment nullable.String  `json:"environment" validate:"max=1024"`
	Framework   serviceFramework `json:"framework"`
	Language    serviceLanguage  `json:"language"`
	Name        nullable.String  `json:"name" validate:"max=1024,pattern=regexpAlphaNumericExt"`
	Node        serviceNode      `json:"node"`
	Runtime     serviceRuntime   `json:"runtime"`
	Version     nullable.String  `json:"version" validate:"max=1024"`
}

type serviceAgent struct {
	EphemeralID nullable.String `json:"ephemeral_id" validate:"max=1024"`
	Name        nullable.String `json:"name" validate:"max=1024"`
	Version     nullable.String `json:"version" validate:"max=1024"`
}

type serviceFramework struct {
	Name    nullable.String `json:"name" validate:"max=1024"`
	Version nullable.String `json:"version" validate:"max=1024"`
}

type serviceLanguage struct {
	Name    nullable.String `json:"name" validate:"max=1024"`
	Version nullable.String `json:"version" validate:"max=1024"`
}

type serviceNode struct {
	Name nullable.String `json:"configured_name" validate:"max=1024"`
}

type serviceRuntime struct {
	Name    nullable.String `json:"name" validate:"max=1024"`
	Version nullable.String `json:"version" validate:"max=1024"`
}

type socket struct {
	RemoteAddress nullable.String `json:"remote_address"`
	Encrypted     nullable.Bool   `json:"encrypted"`
}

type spanCount struct {
	Dropped nullable.Int `json:"dropped"`
	Started nullable.Int `json:"started" validate:"required"`
}

type transaction struct {
	Context        context                       `json:"context"`
	Duration       nullable.Float64              `json:"duration" validate:"required,min=0"`
	ID             nullable.String               `json:"id" validate:"required,max=1024"`
	Marks          map[string]map[string]float64 `json:"marks" validate:"patternKeys=regexpNoDotAsteriskQuote"`
	Name           nullable.String               `json:"name" validate:"max=1024"`
	Outcome        nullable.String               `json:"outcome" validate:"enum=enumOutcome"`
	ParentID       nullable.String               `json:"parent_id" validate:"max=1024"`
	Result         nullable.String               `json:"result" validate:"max=1024"`
	Sampled        nullable.Bool                 `json:"sampled"`
	SampleRate     nullable.Float64              `json:"sample_rate"` //TODO(simitt): how to map to model?
	SpanCount      spanCount                     `json:"span_count" validate:"required"`
	Timestamp      nullable.TimeMicrosUnix       `json:"timestamp"`
	TraceID        nullable.String               `json:"trace_id" validate:"required,max=1024"`
	Type           nullable.String               `json:"type" validate:"required,max=1024"`
	UserExperience userExperience                `json:"experience"`
}

type url struct {
	Full     nullable.String    `json:"full" validate:"max=1024"`
	Hash     nullable.String    `json:"hash" validate:"max=1024"`
	Hostname nullable.String    `json:"hostname" validate:"max=1024"`
	Path     nullable.String    `json:"pathname" validate:"max=1024"`
	Port     nullable.Interface `json:"port" validate:"max=1024,types=string;int"`
	Protocol nullable.String    `json:"protocol" validate:"max=1024"`
	Raw      nullable.String    `json:"raw" validate:"max=1024"`
	Search   nullable.String    `json:"search" validate:"max=1024"`
}

type user struct {
	ID    nullable.Interface `json:"id" validate:"max=1024,types=string;int"`
	Email nullable.String    `json:"email" validate:"max=1024"`
	Name  nullable.String    `json:"username" validate:"max=1024"`
}

// userExperience holds real user (browser) experience metrics.
type userExperience struct {
	// CumulativeLayoutShift holds the Cumulative Layout Shift (CLS) metric value,
	// or a negative value if CLS is unknown. See https://web.dev/cls/
	CumulativeLayoutShift nullable.Float64 `json:"cls" validate:"min=0"`

	// FirstInputDelay holds the First Input Delay (FID) metric value,
	// or a negative value if FID is unknown. See https://web.dev/fid/
	FirstInputDelay nullable.Float64 `json:"fid" validate:"min=0"`

	// TotalBlockingTime holds the Total Blocking Time (TBT) metric value,
	// or a negative value if TBT is unknown. See https://web.dev/tbt/
	TotalBlockingTime nullable.Float64 `json:"tbt" validate:"min=0"`
}
