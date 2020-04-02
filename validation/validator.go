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

package validation

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema"
)

// Error represents an error due to JSON validation.
type Error struct {
	Err error
}

func (e *Error) Error() string {
	return "error validating JSON: " + e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

func CreateSchema(schemaData string, url string) *jsonschema.Schema {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(url, strings.NewReader(schemaData)); err != nil {
		panic(err)
	}
	compiler.Draft = jsonschema.Draft7
	schema, err := compiler.Compile(url)
	if err != nil {
		panic(err)
	}
	return schema
}

// ValidateObject checks that raw is a non-nil, decoded JSON object
// (i.e. has type map[string]interface{}), and validates against the
// provided schema.
func ValidateObject(raw interface{}, schema *jsonschema.Schema) (map[string]interface{}, error) {
	if raw == nil {
		return nil, &Error{errors.New("input missing")}
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil, &Error{errors.New("invalid input type")}
	}
	if err := Validate(raw, schema); err != nil {
		return nil, err
	}
	return obj, nil
}

func Validate(raw interface{}, schema *jsonschema.Schema) error {
	if err := schema.ValidateInterface(raw); err != nil {
		return &Error{err}
	}
	return nil
}
