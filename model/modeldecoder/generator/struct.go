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

func generateStructValidation(w io.Writer, _ []structField, f structField, isCustomStruct bool) error {
	// if field is a custom struct, call its validation function
	if isCustomStruct {
		ruleValidateCustomStruct(w, f)
	}

	// handle generally available rules:
	// - `required`
	// and throw error for others
	vTag, err := validationTag(f.tag)
	if err != nil {
		return err
	}
	rules := validationRules(vTag)
	for _, rule := range rules {
		switch rule.name {
		case tagRequired:
			ruleNullableRequired(w, f)
		default:
			return fmt.Errorf("unhandled tag rule '%s'", rule)
		}
	}
	return nil
}
