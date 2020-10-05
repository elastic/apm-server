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

	"github.com/pkg/errors"
)

var nullableIntValidationFns = validationFunctions{
	vFieldFns: map[string]vFieldFn{
		tagMax:      nintRuleMinMax,
		tagMin:      nintRuleMinMax,
		tagRequired: nintRuleRequired,
	}}

func generateNullableIntValidation(w io.Writer, fields []structField, f structField, isCustomStruct bool) error {
	// if field is a custom struct, call its validation function
	if isCustomStruct {
		ruleValidateCustomStruct(w, f)
	}
	if err := validation(nullableIntValidationFns, w, fields, f); err != nil {
		return errors.Wrap(err, "nullableInt")
	}
	return nil
}

func nintRuleMinMax(w io.Writer, f structField, rule validationRule) error {
	fmt.Fprintf(w, `
if val.%s.Val %s %s {
	return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
}
`[1:], f.Name(), ruleMinMaxOperator(rule.name), rule.value, jsonName(f), rule.name, rule.value)
	return nil
}

func nintRuleRequired(w io.Writer, f structField, rule validationRule) error {
	ruleNullableRequired(w, f)
	return nil
}
