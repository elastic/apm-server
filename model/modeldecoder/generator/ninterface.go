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
	"strings"

	"github.com/pkg/errors"
)

func generateNullableInterfaceValidation(w io.Writer, fields []structField, f structField, _ bool) error {
	rules, err := validationRules(f.tag)
	if err != nil {
		return errors.Wrap(err, "nullableInterface")
	}
	for _, rule := range rules {
		switch rule.name {
		case tagMaxLength, tagTargetType:
			//handled in switch statement for string types
		case tagRequired:
			ruleNullableRequired(w, f)
		case tagInputTypes:
			if err := nullableInterfaceRuleTypes(w, f, rules, rule); err != nil {
				return errors.Wrap(err, "nullableInterface")
			}
		default:
			errors.Wrap(errUnhandledTagRule(rule), "nullableInterface")
		}
	}
	return nil
}

func nullableInterfaceRuleTypes(w io.Writer, f structField, rules []validationRule, rule validationRule) error {
	var isRequired bool
	var maxLengthRule validationRule
	var targetTypeRule validationRule
	var useValue bool
	for _, r := range rules {
		if r.name == tagRequired {
			isRequired = true
			continue
		}
		if r.name == tagMaxLength {
			maxLengthRule = r
			useValue = true
			continue
		}
		if r.name == tagTargetType {
			targetTypeRule = r
			if targetTypeRule.value != "int" {
				return fmt.Errorf("unhandled targetType %s", targetTypeRule.value)
			}
			useValue = true
			continue
		}
	}

	var switchStmt string
	if useValue {
		switchStmt = `switch t := val.%s.Val.(type){`
	} else {
		switchStmt = `switch val.%s.Val.(type){`
	}
	fmt.Fprintf(w, switchStmt, f.Name())

	for _, typ := range strings.Split(rule.value, ";") {
		switch typ {
		case "int":
			fmt.Fprintf(w, `
case int:
case json.Number:
	if _, err := t.Int64(); err != nil{
		return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
	}
`[1:], jsonName(f), rule.name, rule.value)
		case "string":
			fmt.Fprintf(w, `
case %s:
`[1:], typ)
			if maxLengthRule != (validationRule{}) {
				fmt.Fprintf(w, `
if utf8.RuneCountInString(t) %s %s{
	return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
}
`[1:], ruleMinMaxOperator(maxLengthRule.name), maxLengthRule.value, jsonName(f), maxLengthRule.name, maxLengthRule.value)
			}
			if targetTypeRule.value == "int" {
				fmt.Fprintf(w, `
if _, err := strconv.Atoi(t); err != nil{
	return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
}
`[1:], jsonName(f), targetTypeRule.name, targetTypeRule.value)
			}
		case "object":
			fmt.Fprint(w, `
case map[string]interface{}:
`[1:])
		default:
			return fmt.Errorf("unhandled %s %s", rule.name, rule.value)
		}
	}
	if !isRequired {
		fmt.Fprintf(w, `
case nil:
`[1:])
	}
	fmt.Fprintf(w, `
default:
	return fmt.Errorf("'%s': validation rule '%s(%s)' violated ")
}
`[1:], jsonName(f), rule.name, rule.value)
	return nil
}
