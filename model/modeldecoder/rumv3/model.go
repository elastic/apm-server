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
	"regexp"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model/modeldecoder/nullable"
)

var (
	regexpAlphaNumericExt    = regexp.MustCompile("^[a-zA-Z0-9 _-]+$")
	regexpNoDotAsteriskQuote = regexp.MustCompile("^[^.*\"]*$") //do not allow '.' '*' '"'

	enumOutcome = []string{"success", "failure", "unknown"}
)

// entry points

type metadataRoot struct {
	Metadata metadata `json:"m" validate:"required"`
}

type transactionRoot struct {
	Transaction transaction `json:"x" validate:"required"`
}

// other structs

type context struct {
	Custom   common.MapStr   `json:"cu" validate:"patternKeys=regexpNoDotAsteriskQuote"`
	Page     contextPage     `json:"p"`
	Request  contextRequest  `json:"q"`
	Response contextResponse `json:"r"`
	Service  contextService  `json:"se"`
	Tags     common.MapStr   `json:"g" validate:"patternKeys=regexpNoDotAsteriskQuote,typesVals=string;bool;number,maxVals=1024"`
	User     user            `json:"u"`
}

type contextPage struct {
	URL     nullable.String `json:"url"`
	Referer nullable.String `json:"rf"`
}

type contextRequest struct {
	Env         nullable.Interface  `json:"en"`
	Headers     nullable.HTTPHeader `json:"he"`
	HTTPVersion nullable.String     `json:"hve" validate:"max=1024"`
	Method      nullable.String     `json:"mt" validate:"required,max=1024"`
}

type contextResponse struct {
	DecodedBodySize nullable.Float64    `json:"dbs"`
	EncodedBodySize nullable.Float64    `json:"ebs"`
	Headers         nullable.HTTPHeader `json:"he"`
	StatusCode      nullable.Int        `json:"sc"`
	TransferSize    nullable.Float64    `json:"ts"`
}

type contextService struct {
	Agent       contextServiceAgent     `json:"a"`
	Environment nullable.String         `json:"en" validate:"max=1024"`
	Framework   contextServiceFramework `json:"fw"`
	Language    contextServiceLanguage  `json:"la"`
	Name        nullable.String         `json:"n" validate:"max=1024,pattern=regexpAlphaNumericExt"`
	Runtime     contextServiceRuntime   `json:"ru"`
	Version     nullable.String         `json:"ve" validate:"max=1024"`
}

type contextServiceAgent struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type contextServiceFramework struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type contextServiceLanguage struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type contextServiceRuntime struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type metadata struct {
	Labels  common.MapStr   `json:"l" validate:"patternKeys=regexpNoDotAsteriskQuote,typesVals=string;bool;number,maxVals=1024"`
	Service metadataService `json:"se" validate:"required"`
	User    user            `json:"u"`
}

type metadataService struct {
	Agent       metadataServiceAgent     `json:"a" validate:"required"`
	Environment nullable.String          `json:"en" validate:"max=1024"`
	Framework   metadataServiceFramework `json:"fw"`
	Language    metadataServiceLanguage  `json:"la"`
	Name        nullable.String          `json:"n" validate:"required,min=1,max=1024,pattern=regexpAlphaNumericExt"`
	Runtime     metadataServiceRuntime   `json:"ru"`
	Version     nullable.String          `json:"ve" validate:"max=1024"`
}

type metadataServiceAgent struct {
	Name    nullable.String `json:"n" validate:"required,min=1,max=1024"`
	Version nullable.String `json:"ve" validate:"required,max=1024"`
}

type metadataServiceFramework struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type metadataServiceLanguage struct {
	Name    nullable.String `json:"n" validate:"required,max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type metadataServiceRuntime struct {
	Name    nullable.String `json:"n" validate:"required,max=1024"`
	Version nullable.String `json:"ve" validate:"required,max=1024"`
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
	SampleRate     nullable.Float64              `json:"sr"`
	SpanCount      transactionSpanCount          `json:"yc" validate:"required"`
	TraceID        nullable.String               `json:"tid" validate:"required,max=1024"`
	Type           nullable.String               `json:"t" validate:"required,max=1024"`
	UserExperience transactionUserExperience     `json:"exp"`
	Experimental   nullable.Interface            `json:"exper"`
}

type transactionSpanCount struct {
	Dropped nullable.Int `json:"dd"`
	Started nullable.Int `json:"sd" validate:"required"`
}

// userExperience holds real user (browser) experience metrics.
type transactionUserExperience struct {
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

type user struct {
	ID    nullable.Interface `json:"id" validate:"max=1024,types=string;int"`
	Email nullable.String    `json:"em" validate:"max=1024"`
	Name  nullable.String    `json:"un" validate:"max=1024"`
}
