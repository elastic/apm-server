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
	"strings"

	"github.com/pkg/errors"
)

// tmap implements generation logic for creating validation rules
// on map types
type tmap struct {
	supportedTags []string
}

func newTmap() *tmap {
	return &tmap{supportedTags: []string{
		tagMaxVals, tagPatternKeys, tagTypesVals, tagRequired}}
}

func (gen *tmap) validation(w io.Writer, fields []structField, f structField, isCustomStruct bool) error {
	typ, ok := f.Type().Underlying().(*types.Map)
	if !ok {
		// this should never happen
		return errors.New("unexpected field type for tmap")
	}
	// verify that validation rules for map kind exist
	switch typ.Elem().Underlying().(type) {
	case *types.Basic, *types.Interface: // do nothing special
	case *types.Struct:
		if !isCustomStruct {
			return fmt.Errorf("unhandled struct type %s", typ)
		}
	default:
		return fmt.Errorf("unhandled type %s", typ)
	}
	vTag, err := validationTag(f.tag)
	if err != nil {
		return err
	}
	// check if all configured tags are supported
	for k := range vTag {
		var supported bool
		for _, s := range gen.supportedTags {
			if k == s {
				supported = true
				break
			}
		}
		if !supported {
			return fmt.Errorf("unhandled tag rule '%v'", k)
		}
	}

	// validation rules must be run on map itself and its elements
	// 1. apply map validation rules:
	if ruleValue, ok := vTag[tagRequired]; ok {
		gen.ruleRequired(w, f, validationRule{name: tagRequired, value: ruleValue})
		if len(vTag) == 1 {
			return nil
		}
	}
	if len(vTag) == 0 {
		return nil
	}
	// 2. iterate over map and apply validation rules to its elements
	if _, ok := vTag[tagTypesVals]; ok || isCustomStruct {
		fmt.Fprintf(w, `
for k,v := range val.%s{
`[1:], f.Name())
	} else {
		fmt.Fprintf(w, `
for k := range val.%s{
`[1:], f.Name())
	}

	if isCustomStruct {
		// call validation on every item
		fmt.Fprintf(w, `
if err := v.validate(); err != nil{
		return errors.Wrapf(err, "%s")
}
`[1:], jsonName(f))
	}
	if patternKeysValue, ok := vTag[tagPatternKeys]; ok {
		gen.rulePatternKeys(w, f, validationRule{name: tagPatternKeys, value: patternKeysValue})
	}
	if typesValsValue, ok := vTag[tagTypesVals]; ok {
		gen.ruleTypesVals(w, f, vTag, validationRule{name: tagTypesVals, value: typesValsValue})
	}
	// close iteration over map
	fmt.Fprintf(w, `
}
`[1:])
	return nil
}

func (gen *tmap) ruleTypesVals(w io.Writer, f structField, rules map[string]string, rule validationRule) error {
	fmt.Fprintf(w, `
switch t := v.(type){
`[1:])
	// if values are not required allow nil
	if _, ok := rules[tagRequired]; !ok {
		fmt.Fprintf(w, `
case nil:
`[1:])
	}
	for _, typ := range strings.Split(rule.value, ";") {
		if typ == "number" {
			typ = "json.Number"
		}
		fmt.Fprintf(w, `
case %s:
`[1:], typ)
		if typ == "string" {
			if maxValValue, ok := rules[tagMaxVals]; ok {
				gen.ruleMaxVals(w, f, validationRule{name: tagMaxVals, value: maxValValue})
			}
		}
	}
	fmt.Fprintf(w, `
default:
	return fmt.Errorf("'%s': validation rule '%s(%s)' violated for key %%s",k)
}
`[1:], jsonName(f), rule.name, rule.value)
	return nil
}

func (gen *tmap) ruleRequired(w io.Writer, f structField, rule validationRule) error {
	fmt.Fprintf(w, `
if len(val.%s) == 0{
	return fmt.Errorf("'%s' required")
}
`[1:], f.Name(), jsonName(f))
	return nil
}

func (gen *tmap) rulePatternKeys(w io.Writer, f structField, rule validationRule) error {
	fmt.Fprintf(w, `
if k != "" && !%s.MatchString(k){
		return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
}
`[1:], rule.value, jsonName(f), rule.name, rule.value)
	return nil
}

func (gen *tmap) ruleMaxVals(w io.Writer, f structField, rule validationRule) error {
	fmt.Fprintf(w, `
if utf8.RuneCountInString(t) > %s{
	return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
}
`[1:], rule.value, jsonName(f), rule.name, rule.value)
	return nil
}
