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

	"github.com/elastic/apm-server/model/modeldecoder/typ"
	"github.com/elastic/beats/v7/libbeat/common"
)

var (
	alphaNumericExtRegex = regexp.MustCompile("^[a-zA-Z0-9 _-]+$")
	labelsRegex          = regexp.MustCompile("^[^.*\"]*$") //do not allow '.' '*' '"'
)

// metadata contain event metadata
type metadata struct {
	Labels  common.MapStr   `json:"l" validate:"patternKeys=labelsRegex,typesVals=string;bool;number,maxVals=1024"`
	Service metadataService `json:"se" validate:"required"`
	User    metadataUser    `json:"u"`
}

// metadataService holds information about where the data was collected
type metadataService struct {
	Agent       metadataServiceAgent     `json:"a" validate:"required"`
	Environment typ.String               `json:"en" validate:"max=1024"`
	Framework   MetadataServiceFramework `json:"fw"`
	Language    metadataServiceLanguage  `json:"la"`
	Name        typ.String               `json:"n" validate:"required,max=1024,pattern=alphaNumericExtRegex"`
	Runtime     metadataServiceRuntime   `json:"ru"`
	Version     typ.String               `json:"ve" validate:"max=1024"`
}

//metadataServiceAgent has a version and a name
type metadataServiceAgent struct {
	Name    typ.String `json:"n" validate:"required,max=1024"`
	Version typ.String `json:"ve" validate:"required,max=1024"`
}

//MetadataServiceFramework has a version and name
type MetadataServiceFramework struct {
	Name    typ.String `json:"n" validate:"max=1024"`
	Version typ.String `json:"ve" validate:"max=1024"`
}

//MetadataLanguage has a version and name
type metadataServiceLanguage struct {
	Name    typ.String `json:"n" validate:"required,max=1024"`
	Version typ.String `json:"ve" validate:"max=1024"`
}

//MetadataRuntime has a version and name
type metadataServiceRuntime struct {
	Name    typ.String `json:"n" validate:"required,max=1024"`
	Version typ.String `json:"ve" validate:"required,max=1024"`
}

type metadataUser struct {
	ID    typ.Interface `json:"id" validate:"max=1024,types=string;int"`
	Email typ.String    `json:"em" validate:"max=1024"`
	Name  typ.String    `json:"un" validate:"max=1024"`
}

type metadataWithKey struct {
	Metadata metadata `json:"m" validate:"required"`
}
