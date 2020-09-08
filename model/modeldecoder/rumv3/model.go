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
)

type metadataRoot struct {
	Metadata metadata `json:"m" validate:"required"`
}

type metadata struct {
	Labels  common.MapStr   `json:"l" validate:"patternKeys=regexpNoDotAsteriskQuote,typesVals=string;bool;number,maxVals=1024"`
	Service metadataService `json:"se" validate:"required"`
	User    metadataUser    `json:"u"`
}

type metadataService struct {
	Agent       metadataServiceAgent     `json:"a" validate:"required"`
	Environment nullable.String          `json:"en" validate:"max=1024"`
	Framework   MetadataServiceFramework `json:"fw"`
	Language    metadataServiceLanguage  `json:"la"`
	Name        nullable.String          `json:"n" validate:"required,min=1,max=1024,pattern=regexpAlphaNumericExt"`
	Runtime     metadataServiceRuntime   `json:"ru"`
	Version     nullable.String          `json:"ve" validate:"max=1024"`
}

type metadataServiceAgent struct {
	Name    nullable.String `json:"n" validate:"required,min=1,max=1024"`
	Version nullable.String `json:"ve" validate:"required,max=1024"`
}

type MetadataServiceFramework struct {
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

type metadataUser struct {
	ID    nullable.Interface `json:"id" validate:"max=1024,types=string;int"`
	Email nullable.String    `json:"em" validate:"max=1024"`
	Name  nullable.String    `json:"un" validate:"max=1024"`
}
