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

// nstring implements generation logic for creating validation rules
// on nullable.String types
type nstring struct {
	validationFns validationFunctions
	imports       map[string]struct{}
}

func newNstring(imports map[string]struct{}) *nstring {
	gen := nstring{imports: imports}
	gen.validationFns = validationFunctions{
		vFieldFns: map[string]vFieldFn{
			tagEnum:     gen.ruleEnum,
			tagMax:      gen.ruleMinMax,
			tagMin:      gen.ruleMinMax,
			tagPattern:  gen.rulePattern,
			tagRequired: gen.ruleRequired},
		vStructFns: map[string]vStructFn{
			tagRequiredIfAny: gen.ruleRequiredIfAny}}
	return &gen
}

func (gen *nstring) validation(w io.Writer, fields []structField, f structField) error {
	err := validation(gen.validationFns, w, fields, f)
	if err != nil {
		return errors.Wrap(err, "nullableString")
	}
	return nil
}

func (gen *nstring) ruleEnum(w io.Writer, f structField, rule validationRule) error {
	fmt.Fprintf(w, `
if val.%s.Val != ""{
	var matchEnum bool
	for _, s := range %s {
		if val.%s.Val == s{
			matchEnum = true
			break
		}
	}
	if !matchEnum{
		return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
	}
}
`[1:], f.Name(), rule.value, f.Name(), jsonName(f), rule.name, rule.value)
	return nil
}

func (gen *nstring) ruleMinMax(w io.Writer, f structField, rule validationRule) error {
	gen.imports[importUTF8] = struct{}{}
	fmt.Fprintf(w, `
if utf8.RuneCountInString(val.%s.Val) %s %s{
	return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
}
`[1:], f.Name(), ruleMinMaxOperator(rule.name), rule.value, jsonName(f), rule.name, rule.value)
	return nil
}

func (gen *nstring) rulePattern(w io.Writer, f structField, rule validationRule) error {
	fmt.Fprintf(w, `
if val.%s.Val != "" && !%s.MatchString(val.%s.Val){
	return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
}
`[1:], f.Name(), rule.value, f.Name(), jsonName(f), rule.name, rule.value)
	return nil
}

func (gen *nstring) ruleRequired(w io.Writer, f structField, rule validationRule) error {
	ruleNullableRequired(w, f)
	return nil
}

func (gen *nstring) ruleRequiredIfAny(w io.Writer, fields []structField, f structField, rule validationRule) error {
	return ruleRequiredIfAny(w, fields, f, rule.value)
}
