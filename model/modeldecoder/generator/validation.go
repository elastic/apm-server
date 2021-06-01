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
	tagEnum           = "enum"
	tagInputTypes     = "inputTypes"
	tagInputTypesVals = "inputTypesVals"
	tagMax            = "max"
	tagMaxLength      = "maxLength"
	tagMaxLengthVals  = "maxLengthVals"
	tagMin            = "min"
	tagMinLength      = "minLength"
	tagMinVals        = "minVals"
	tagPattern        = "pattern"
	tagPatternKeys    = "patternKeys"
	tagRequired       = "required"
	tagRequiredAnyOf  = "requiredAnyOf"
	tagRequiredIfAny  = "requiredIfAny"
	tagTargetType     = "targetType"
)

type validationRule struct {
	name  string
	value string
}

func errUnhandledTagRule(rule validationRule) error {
	return fmt.Errorf("unhandled tag rule '%s'", rule.name)
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

func validationRules(structTag reflect.StructTag) ([]validationRule, error) {
	tag, err := validationTag(structTag)
	if err != nil {
		return nil, err
	}
	var rules = make([]validationRule, 0, len(tag))
	for k, v := range tag {
		rules = append(rules, validationRule{name: k, value: v})
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].name < rules[j].name
	})
	return rules, nil
}

func ruleMinMaxOperator(ruleName string) string {
	switch ruleName {
	case tagMin, tagMinLength, tagMinVals:
		return "<"
	case tagMax, tagMaxLength:
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
	oneOf, err := filteredFields(fields, strings.Split(tagValue, ";"))
	if err != nil {
		return err
	}
	if len(oneOf) <= 1 {
		return fmt.Errorf("invalid usage of rule 'requiredOneOf' - try 'required' instead")
	}

	fmt.Fprintf(w, `if `)
	for i, oneOfField := range oneOf {
		if i > 0 {
			fmt.Fprintf(w, " && ")
		}
		fmt.Fprint(w, "!")
		if err := generateIsSet(w, oneOfField, "val."); err != nil {
			return err
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
	ifAny, err := filteredFields(fields, strings.Split(tagValue, ";"))
	if err != nil {
		return err
	}

	// Only check ifAny fields if the field itself is not set
	fmt.Fprint(w, "if !")
	if err := generateIsSet(w, field, "val."); err != nil {
		return err
	}
	fmt.Fprintln(w, " {")

	// Check if any of the fields is set. We create a separate "if" block
	// for each field so we can include its name in the error.
	for _, ifAnyField := range ifAny {
		fmt.Fprint(w, "if ")
		if err := generateIsSet(w, ifAnyField, "val."); err != nil {
			return err
		}
		fmt.Fprintf(w, ` {
	return fmt.Errorf("'%s' required when '%s' is set")
}
`, jsonName(field), jsonName(ifAnyField))
	}

	fmt.Fprintln(w, "}")
	return nil
}

func filteredFields(fields []structField, jsonNames []string) ([]structField, error) {
	mapped := make(map[string]structField)
	for _, field := range fields {
		mapped[jsonName(field)] = field
	}
	filtered := make([]structField, len(jsonNames))
	for i, jsonName := range jsonNames {
		field, ok := mapped[jsonName]
		if !ok {
			return nil, fmt.Errorf("unknown field name %q", jsonName)
		}
		filtered[i] = field
	}
	return filtered, nil
}
