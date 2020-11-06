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

package generatortest

//lint:file-ignore U1000 Ignore all unused code, it's used for json schema code generation in tests

import (
	"github.com/elastic/apm-server/model/modeldecoder/nullable"
	"github.com/elastic/beats/libbeat/common"
)

var (
	patternA = `^[^."]*$`
	patternB = `^[ab]+$`
	patternC = `^[c]+$`
	enumA    = []string{"open", "closed"}
)

type String struct {
	Required     nullable.String `json:"required" validate:"required,enum=enumA"`
	NullableEnum nullable.String `json:"nullable_enum" validate:"enum=enumA"`
	Nullable     nullable.String `json:"nullable" validate:"maxLength=5,minLength=2,pattern=patternA"`
}

type Number struct {
	Required nullable.Int     `json:"required" validate:"required,max=250,min=1"`
	Nullable nullable.Float64 `json:"nullable" validate:"max=15.9,min=0.5"`
}

type Bool struct {
	Required nullable.Bool `json:"required" validate:"required"`
	Nullable nullable.Bool `json:"nullable"`
}

type HTTPHeader struct {
	Required nullable.HTTPHeader `json:"required" validate:"required"`
	Nullable nullable.HTTPHeader `json:"nullable"`
}

type Interface struct {
	Required nullable.Interface `json:"required" validate:"required,inputTypes=string;int;float64;bool;map[string]interface,maxLength=5,minLength=2,pattern=patternA,max=250,min=1.5"`
	Nullable nullable.Interface `json:"nullable" validate:"inputTypes=string,enum=enumA"`
}

type Map struct {
	Required  common.MapStr        `json:"required" validate:"required,inputTypesVals=string;bool;number,maxLengthVals=5,patternKeys=patternB"`
	Nullable  common.MapStr        `json:"nullable"`
	Nested    map[string]NestedMap `json:"nested_a" validate:"patternKeys=patternB"`
	StructMap NestedStruct         `json:"nested_b"`
}

type NestedMap struct {
	Required nullable.Float64 `json:"required" validate:"required"`
}

type NestedStruct struct {
	A map[string]NestedStructMap `json:"-" validate:"patternKeys=patternB"`
}

type NestedStructMap struct {
	B map[string]string `json:"-" validate:"patternKeys=patternC"`
}

type Slice struct {
	Strings  []string `json:"required" validate:"required,maxLength=3,minLength=2"`
	Nullable []string `json:"nullable" validate:"pattern=patternB"`
	Children []SliceA `json:"children"`
}

type SliceA struct {
	Number nullable.Float64 `json:"number"`
	Slices []SliceA         `json:"children"`
}

type RequiredIfAny struct {
	A nullable.String `json:"a" validate:"requiredIfAny=b;c"`
	B nullable.String `json:"b"`
	C nullable.String `json:"c" validate:"requiredIfAny=b"`
}

type RequiredAnyOf struct {
	A nullable.Int
	B nullable.Int
	_ struct{} `validate:"requiredAnyOf=a;b"`
}

type Exported struct {
	a nullable.Int
	_ nullable.Int
	B nullable.Int
}
