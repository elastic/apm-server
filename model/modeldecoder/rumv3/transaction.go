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

package rumv3

import (
	"github.com/elastic/apm-server/model/modeldecoder/nullable"
	"github.com/elastic/beats/v7/libbeat/common"
)

var (
	enumOutcome = []string{"success", "failure", "unknown"}
)

type transactionRoot struct {
	Transaction transaction `json:"x" validate:"required"`
}

type context struct {
	Custom   common.MapStr `json:"cu" validate:"patternKeys=regexpNoDotAsteriskQuote"`
	Page     page          `json:"p"`
	Response response      `json:"r"`
	Request  request       `json:"q"`
	Service  service       `json:"se"`
	Tags     common.MapStr `json:"g" validate:"patternKeys=regexpNoDotAsteriskQuote,typesVals=string;bool;number,maxVals=1024"`
	User     user          `json:"u"`
}

//TODO(simitt): implement
// type metricset struct {
// }

type page struct {
	URL     nullable.String `json:"url"`
	Referer nullable.String `json:"rf"`
}

type queue struct {
	Name nullable.String `json:"name" validate:"max=1024"`
}

type request struct {
	Env         nullable.Interface  `json:"en"`
	Headers     nullable.HTTPHeader `json:"he"`
	HTTPVersion nullable.String     `json:"hve" validate:"max=1024"`
	Method      nullable.String     `json:"mt" validate:"required,max=1024"`
}

type response struct {
	DecodedBodySize nullable.Float64    `json:"dbs"`
	EncodedBodySize nullable.Float64    `json:"ebs"`
	Headers         nullable.HTTPHeader `json:"he"`
	StatusCode      nullable.Int        `json:"sc"`
	TransferSize    nullable.Float64    `json:"ts"`
}

type service struct {
	Agent       serviceAgent     `json:"a"`
	Environment nullable.String  `json:"en" validate:"max=1024"`
	Framework   serviceFramework `json:"fw"`
	Language    serviceLanguage  `json:"la"`
	Name        nullable.String  `json:"n" validate:"max=1024,pattern=regexpAlphaNumericExt"`
	Runtime     serviceRuntime   `json:"ru"`
	Version     nullable.String  `json:"ve" validate:"max=1024"`
}

type serviceAgent struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type serviceFramework struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type serviceLanguage struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type serviceRuntime struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

//TODO(simitt): implement
// type span struct {
// ID nullable.String `json:"id" validate:"max=1024"`
// }

type spanCount struct {
	Dropped nullable.Int `json:"dd"`
	Started nullable.Int `json:"sd" validate:"required"`
}

type transaction struct {
	Context        context                       `json:"c"`
	Duration       nullable.Float64              `json:"d" validate:"required,min=0"`
	ID             nullable.String               `json:"id" validate:"required,max=1024"`
	Marks          map[string]map[string]float64 `json:"k" validate:"patternKeys=regexpNoDotAsteriskQuote"`
	Name           nullable.String               `json:"n" validate:"max=1024"`
	Outcome        nullable.String               `json:"o" validate:"enum=enumOutcome"`
	ParentID       nullable.String               `json:"pid" validate:"max=1024"`
	Result         nullable.String               `json:"rt" validate:"max=1024"`
	Sampled        nullable.Bool                 `json:"sm"`
	SampleRate     nullable.Float64              `json:"sr"` //TODO(simitt): how to map to model?
	SpanCount      spanCount                     `json:"yc" validate:"required"`
	TraceID        nullable.String               `json:"tid" validate:"required,max=1024"`
	Type           nullable.String               `json:"t" validate:"required,max=1024"`
	UserExperience userExperience                `json:"exp"`
	// Span           span                          `json:"y"`
	// Metricset      metricset                     `json:"me"`
}

type user struct {
	ID    nullable.Interface `json:"id" validate:"max=1024,types=string;int"`
	Email nullable.String    `json:"em" validate:"max=1024"`
	Name  nullable.String    `json:"un" validate:"max=1024"`
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
