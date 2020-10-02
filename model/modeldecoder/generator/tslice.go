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
	"go/types"
	"io"
)

// tslice implements generation logic for creating validation rules
// on slice types
type tslice struct {
	validationFns validationFunctions
	imports       map[string]struct{}
}

func newTslice(imports map[string]struct{}) *tslice {
	gen := tslice{imports: imports}
	gen.validationFns = validationFunctions{
		vFieldFns: map[string]vFieldFn{
			tagMax:      gen.ruleMinMax,
			tagRequired: gen.ruleRequired},
		vStructFns: map[string]vStructFn{
			tagRequiredOneOf: gen.ruleRequiredOneOf}}
	return &gen
}

func (gen *tslice) validation(w io.Writer, fields []structField, f structField, isCustomStruct bool) error {
	// call validation on every slice element when elements are of custom type
	if isCustomStruct {
		gen.imports[importErrors] = struct{}{}
		fmt.Fprintf(w, `
for _, elem := range val.%s{
	if err := elem.validate(); err != nil{
		return errors.Wrapf(err, "%s")
	}
}
`[1:], f.Name(), jsonName(f))
	}
	// handle configured validation rules
	return validation(gen.validationFns, w, fields, f)
}

func (gen *tslice) ruleMinMax(w io.Writer, f structField, rule validationRule) error {
	sliceT, ok := f.Type().Underlying().(*types.Slice)
	if !ok {
		return fmt.Errorf("unexpected error handling %s for slice", rule.name)
	}
	if basic, ok := sliceT.Elem().Underlying().(*types.Basic); ok {
		if basic.Kind() == types.String {
			gen.imports[importUTF8] = struct{}{}
			fmt.Fprintf(w, `
for _, elem := range val.%s{
	if utf8.RuneCountInString(elem) %s %s{
			return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
	}
}
`[1:], f.Name(), ruleMinMaxOperator(rule.name), rule.value, jsonName(f), rule.name, rule.value)
			return nil
		}
	}
	return fmt.Errorf("unhandled tag rule max for type %s", f.Type().Underlying())
}

func (gen *tslice) ruleRequired(w io.Writer, f structField, rule validationRule) error {
	fmt.Fprintf(w, `
if len(val.%s) == 0{
	return fmt.Errorf("'%s' required")
}
`[1:], f.Name(), jsonName(f))
	return nil
}

func (gen *tslice) ruleRequiredOneOf(w io.Writer, fields []structField, f structField, rule validationRule) error {
	return ruleRequiredOneOf(w, fields, rule.value)
}
