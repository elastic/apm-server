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
	"encoding/json"
	"fmt"
	"go/types"
	"io"

	"github.com/pkg/errors"
)

func generateSliceValidation(w io.Writer, fields []structField, f structField, isCustomStruct bool) error {
	// call validation on every slice element when elements are of custom type
	if isCustomStruct {
		fmt.Fprintf(w, `
for _, elem := range val.%s{
	if err := elem.validate(); err != nil{
		return errors.Wrapf(err, "%s")
	}
}
`[1:], f.Name(), jsonName(f))
	}
	// handle configured validation rules
	rules, err := validationRules(f.tag)
	if err != nil {
		return errors.Wrap(err, "slice")
	}
	for _, rule := range rules {
		switch rule.name {
		case tagMinLength, tagMaxLength:
			err = sliceRuleMinMaxLength(w, f, rule)
		case tagMinVals:
			err = sliceRuleMinVals(w, f, rule)
		case tagRequired:
			sliceRuleRequired(w, f, rule)
		case tagRequiredAnyOf:
			err = ruleRequiredOneOf(w, fields, rule.value)
		case tagRequiredIfAny:
			err = ruleRequiredIfAny(w, fields, f, rule.value)
		default:
			return errors.Wrap(errUnhandledTagRule(rule), "slice")
		}
		if err != nil {
			return errors.Wrap(err, "slice")
		}
	}
	return nil
}

func sliceRuleMinMaxLength(w io.Writer, f structField, rule validationRule) error {
	sliceT, ok := f.Type().Underlying().(*types.Slice)
	if !ok {
		return fmt.Errorf("unexpected error handling %s for slice", rule.name)
	}
	if basic, ok := sliceT.Elem().Underlying().(*types.Basic); ok {
		if basic.Kind() == types.String {
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

func sliceRuleMinVals(w io.Writer, f structField, rule validationRule) error {
	fmt.Fprintf(w, `
for _, elem := range val.%s{
	if elem %s %s{
		return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
	}
}
`[1:], f.Name(), ruleMinMaxOperator(rule.name), rule.value, jsonName(f), rule.name, rule.value)
	return nil
}

func sliceRuleRequired(w io.Writer, f structField, rule validationRule) {
	fmt.Fprintf(w, `
if len(val.%s) == 0{
	return fmt.Errorf("'%s' required")
}
`[1:], f.Name(), jsonName(f))
}

func generateJSONPropertySlice(info *fieldInfo, parent *property, child *property) error {
	child.Type.add(TypeNameArray)
	var minItems int
	if child.Type.required {
		minItems = 1
	}
	child.MinItems = &minItems
	parent.Properties[jsonSchemaName(info.field)] = child

	itemType := info.field.Type().Underlying().(*types.Slice).Elem()
	if _, ok := info.parsed.structTypes[itemType.String()]; ok {
		// parsed struct - no type specific tags will be handled
		return nil
	}
	// non-parsed struct
	// check if type is known, otherwise raise unhandled error
	itemsType, ok := propertyTypes[itemType.String()]
	if !ok {
		return fmt.Errorf("unhandled type %T", itemType)
	}
	// NOTE(simi): set required=true to be aligned with previous JSON schema definitions
	items := property{Type: &propertyType{names: []propertyTypeName{itemsType}, required: true}}
	switch itemsType {
	case TypeNameInteger:
		setPropertyRulesInteger(info, &items)
	case TypeNameNumber:
		setPropertyRulesNumber(info, &items)
	case TypeNameString:
		setPropertyRulesString(info, &items)
	default:
		return fmt.Errorf("unhandled slice item type %s", itemsType)
	}
	if minVals, ok := info.tags[tagMinVals]; ok {
		items.Min = json.Number(minVals)
		delete(info.tags, tagMinVals)
	}
	child.Items = &items
	return nil
}
