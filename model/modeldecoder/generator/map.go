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
	"strings"
)

var mapSupportedTags = []string{tagMaxLengthVals, tagPatternKeys, tagRequired, tagInputTypesVals}

func generateMapValidation(w io.Writer, fields []structField, f structField, isCustomStruct bool) error {
	typ := f.Type().Underlying().(*types.Map)
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
		for _, s := range mapSupportedTags {
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
		mapRuleRequired(w, f, validationRule{name: tagRequired, value: ruleValue})
		if len(vTag) == 1 {
			return nil
		}
	}
	if len(vTag) == 0 {
		return nil
	}
	// 2. iterate over map and apply validation rules to its elements
	if _, ok := vTag[tagInputTypesVals]; ok || isCustomStruct {
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
		mapRulePatternKeys(w, f, validationRule{name: tagPatternKeys, value: patternKeysValue})
	}
	if typesValsValue, ok := vTag[tagInputTypesVals]; ok {
		mapRuleTypesVals(w, f, vTag, validationRule{name: tagInputTypesVals, value: typesValsValue})
	}
	fmt.Fprintf(w, `
}
`[1:])
	return nil
}

func mapRuleTypesVals(w io.Writer, f structField, rules map[string]string, rule validationRule) {
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
			if maxValValue, ok := rules[tagMaxLengthVals]; ok {
				mapRuleMaxVals(w, f, validationRule{name: tagMaxLengthVals, value: maxValValue})
			}
		}
	}
	fmt.Fprintf(w, `
default:
	return fmt.Errorf("'%s': validation rule '%s(%s)' violated for key %%s",k)
}
`[1:], jsonName(f), rule.name, rule.value)
}

func mapRuleRequired(w io.Writer, f structField, rule validationRule) {
	fmt.Fprintf(w, `
if len(val.%s) == 0{
	return fmt.Errorf("'%s' required")
}
`[1:], f.Name(), jsonName(f))
}

func mapRulePatternKeys(w io.Writer, f structField, rule validationRule) {
	fmt.Fprintf(w, `
if k != "" && !%sRegexp.MatchString(k){
		return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
}
`[1:], rule.value, jsonName(f), rule.name, rule.value)
}

func mapRuleMaxVals(w io.Writer, f structField, rule validationRule) {
	fmt.Fprintf(w, `
if utf8.RuneCountInString(t) > %s{
	return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
}
`[1:], rule.value, jsonName(f), rule.name, rule.value)
}

func generateJSONPropertyMap(info *fieldInfo, parent *property, child *property, nested *property) error {
	name := jsonSchemaName(info.field)
	child.Type.add(TypeNameObject)
	patternName, isPatternProp := info.tags[tagPatternKeys]
	delete(info.tags, tagPatternKeys)
	if !isPatternProp && name == "" {
		// the object (map) can be places as a property with a defined key,
		// or as a patternProperty with a defined key pattern
		// if both are missing the field cannot be added
		return fmt.Errorf("invalid combination: either json name or tag %s must be given", tagPatternKeys)
	}
	if !isPatternProp {
		// if no pattern property is given, the child map will be nested directly as
		// property inside the parent property, identified by it's json name
		//   e.g. {"parent":{"properties":{"jsonNameXY":{..}}}}
		*nested = *child
		parent.Properties[name] = nested
	}
	if maxLen, ok := info.tags[tagMaxLengthVals]; ok {
		nested.MaxLength = json.Number(maxLen)
		delete(info.tags, tagMaxLengthVals)
	}
	if inputTypes, ok := info.tags[tagInputTypesVals]; ok {
		names, err := propertyTypesFromTag(tagInputTypesVals, inputTypes)
		if err != nil {
			return err
		}
		delete(info.tags, tagInputTypesVals)
		nested.Type = &propertyType{names: names}
	}
	if !isPatternProp {
		// nothing more to do when no key pattern is given
		return nil
	}
	pattern, ok := info.parsed.patternVariables[patternName]
	if !ok {
		return fmt.Errorf("unhandled %s tag value %s", tagPatternKeys, pattern)
	}
	// for map key patterns two options are supported:
	// - the map does not have a json name defined, in which case it is nested directly
	//   inside the parent object's patternProperty
	//   e.g. {"parent":{"patternProperties":{"patternXY":{..}}}}
	// - the map does have a json name defined, in which case it is nested as
	//   patternProperty inside an object, which itself is nested
	//   inside the parent property, identified by it's json name
	//   e.g. {"parent":{"properties":{"jsonNameXY":{"patternProperties":{"patternXY":{..}}}}}}
	if name == "" {
		if parent.PatternProperties == nil {
			parent.PatternProperties = make(map[string]*property)
		}
		valueType := info.field.Type().Underlying().(*types.Map).Elem()
		typeName, ok := propertyTypes[valueType.String()]
		if !ok {
			typeName = TypeNameObject
		}
		nested.Type = &propertyType{names: []propertyTypeName{typeName}}
		parent.PatternProperties[pattern] = nested
		parent.AdditionalProperties = new(bool)
		return nil
	}
	if child.PatternProperties == nil {
		child.PatternProperties = make(map[string]*property)
	}
	child.PatternProperties[pattern] = nested
	child.AdditionalProperties = new(bool)
	parent.Properties[name] = child
	return nil
}
