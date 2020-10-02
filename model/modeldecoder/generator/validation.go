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
	"reflect"
	"sort"
	"strings"
)

const (
	tagEnum          = "enum"
	tagMax           = "max"
	tagMaxVals       = "maxVals"
	tagMin           = "min"
	tagPattern       = "pattern"
	tagPatternKeys   = "patternKeys"
	tagRequired      = "required"
	tagRequiredOneOf = "requiredOneOf"
	tagRequiredIfAny = "requiredIfAny"
	tagTypes         = "types"
	tagTypesVals     = "typesVals"

	importJSON   = "encoding/json"
	importUTF8   = "unicode/utf8"
	importErrors = "github.com/pkg/errors"
)

// vFieldFn should be used for implementing field specific validation rules,
// independend of other fields or tags
type vFieldFn func(io.Writer, structField, validationRule) error

// vFieldTagsFn should be used for implementing field specific validation rules,
// dependend on other tags, but independend of other fields
type vFieldTagsFn func(io.Writer, structField, []validationRule, validationRule) error

// vStructFn should be used for implementing struct specific validation rules,
// depending on multiplefields
type vStructFn func(io.Writer, []structField, structField, validationRule) error

type validationFunctions struct {
	vFieldFns     map[string]vFieldFn
	vFieldTagsFns map[string]vFieldTagsFn
	vStructFns    map[string]vStructFn
}

type validationRule struct {
	name  string
	value string
}

func validation(validationFns validationFunctions, w io.Writer, fields []structField, f structField) error {
	vTag, err := validationTag(f.tag)
	if err != nil {
		return err
	}
	rules := validationRules(vTag)
	for _, rule := range rules {
		if validationFns.vFieldFns != nil {
			if fn, ok := validationFns.vFieldFns[rule.name]; ok {
				if err := fn(w, f, rule); err != nil {
					return err
				}
				continue
			}
		}
		if validationFns.vFieldTagsFns != nil {
			if fn, ok := validationFns.vFieldTagsFns[rule.name]; ok {
				if err := fn(w, f, rules, rule); err != nil {
					return err
				}
				continue
			}
		}
		if validationFns.vStructFns != nil {
			if fn, ok := validationFns.vStructFns[rule.name]; ok {
				if err := fn(w, fields, f, rule); err != nil {
					return err
				}
				continue
			}
		}
		return fmt.Errorf("unhandled tag rule '%s'", rule)
	}
	return nil
}

func validationTag(structTag reflect.StructTag) (map[string]string, error) {
	parts := parseTag(structTag, "validate")
	m := make(map[string]string, len(parts))
	errPrefix := "parse validation tag:"
	for _, rule := range parts {
		parts := strings.Split(rule, "=")
		switch len(parts) {
		case 1:
			// valueless rule e.g. required
			if rule != parts[0] {
				return nil, fmt.Errorf("%s malformed tag '%s'", errPrefix, rule)
			}
			switch rule {
			case tagRequired:
				m[rule] = ""
			default:
				return nil, fmt.Errorf("%s unhandled tag rule '%s'", errPrefix, rule)
			}
		case 2:
			// rule=value
			m[parts[0]] = parts[1]
		default:
			return nil, fmt.Errorf("%s malformed tag '%s'", errPrefix, rule)
		}
	}
	return m, nil
}

func validationRules(unsorted map[string]string) []validationRule {
	var rules = make([]validationRule, 0, len(unsorted))
	for k, v := range unsorted {
		rules = append(rules, validationRule{name: k, value: v})
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].name < rules[j].name
	})
	return rules
}

func ruleMinMaxOperator(ruleName string) string {
	switch ruleName {
	case tagMin:
		return "<"
	case tagMax:
		return ">"
	default:
		panic("unexpected rule: " + ruleName)
	}
}

//
// common validation rules independend of type
//

func ruleNullableRequired(w io.Writer, f structField) {
	fmt.Fprintf(w, `
if !val.%s.IsSet()  {
	return fmt.Errorf("'%s' required")
}
`[1:], f.Name(), jsonName(f))
}

func ruleRequiredOneOf(w io.Writer, fields []structField, tagValue string) error {
	oneOf := strings.Split(tagValue, ";")
	if len(oneOf) <= 1 {
		return fmt.Errorf("invalid usage of rule 'requiredOneOf' - try 'required' instead")
	}
	fmt.Fprintf(w, `if `)
	var matched bool
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		jName := jsonName(f)
		if j := indexOf(oneOf, jName); j != -1 {
			if matched {
				fmt.Fprintf(w, ` && `)
			}
			fmt.Fprintf(w, ` !val.%s.IsSet()`[1:], f.Name())
			matched = true
			// remove from ifAny names and check if we can return early
			oneOf = append(oneOf[:j], oneOf[j+1:]...)
			if len(oneOf) == 0 {
				break
			}
		}
	}
	fmt.Fprintf(w, ` {
  return fmt.Errorf("requires at least one of the fields '%v'")
}
`[1:], tagValue)
	if len(oneOf) != 0 {
		return fmt.Errorf("unhandled 'requiredOneOf' field name(s)")
	}
	return nil
}

func ruleRequiredIfAny(w io.Writer, fields []structField, field structField, tagValue string) error {
	ifAny := make(map[string]struct{})
	for _, n := range strings.Split(tagValue, ";") {
		ifAny[n] = struct{}{}
	}
	// only check ifAny fields if the field itself is not set
	fmt.Fprintf(w, `
if !val.%s.IsSet()  {
`[1:], field.Name())
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		jName := jsonName(f)
		if _, ok := ifAny[jName]; ok {
			fmt.Fprintf(w, `
if val.%s.IsSet()  {
	return fmt.Errorf("'%s' required when '%s' is set")
}
`[1:], f.Name(), jsonName(field), jsonName(f))
			// remove from ifAny names and check if we can return early
			delete(ifAny, jName)
			if len(ifAny) == 0 {
				break
			}
		}
	}
	if len(ifAny) != 0 {
		return fmt.Errorf("unhandled 'requiredIfAny' field name(s) for %s", field.Name())
	}
	fmt.Fprintf(w, `
}
`[1:])
	return nil
}

func indexOf(s []string, key string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == key {
			return i
		}
	}
	return -1
}
