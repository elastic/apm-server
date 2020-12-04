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

package generator

import (
	"fmt"
	"io"
)

func generateStructValidation(w io.Writer, fields []structField, f structField, isCustomStruct bool) error {
	// if field is a custom struct, call its validation function
	if isCustomStruct {
		fmt.Fprintf(w, `
		if err := val.%s.validate(); err != nil{
			return errors.Wrapf(err, "%s")
		}
		`[1:], f.Name(), jsonName(f))
	}

	// handle generally available rules:
	// - `required`
	// and throw error for others
	rules, err := validationRules(f.tag)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		switch rule.name {
		case tagRequired:
			ruleNullableRequired(w, f)
		case tagRequiredAnyOf:
			ruleRequiredOneOf(w, fields, rule.value)
		default:
			return fmt.Errorf("unhandled tag rule '%s'", rule)
		}
	}
	return nil
}

func generateJSONPropertyStruct(info *fieldInfo, parent *property, child *property) error {
	child.Type.add(TypeNameObject)
	parent.Properties[jsonSchemaName(info.field)] = child
	return nil
}
