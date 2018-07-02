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

package processor

import (
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema"
)

func CreateSchema(schemaData string, url string) *jsonschema.Schema {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(url, strings.NewReader(schemaData)); err != nil {
		panic(err)
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		panic(err)
	}
	return schema
}

func Validate(raw interface{}, schema *jsonschema.Schema) error {
	if err := schema.ValidateInterface(raw); err != nil {
		return fmt.Errorf("Problem validating JSON document against schema: %v", err)
	}
	return nil
}
