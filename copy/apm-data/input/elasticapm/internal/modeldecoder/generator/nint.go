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

func generateNullableIntValidation(w io.Writer, fields []structField, f structField, _ bool) error {
	rules, err := validationRules(f.tag)
	if err != nil {
		return errors.Wrap(err, "nullableInt")
	}
	for _, rule := range rules {
		switch rule.name {
		case tagMin, tagMax:
			nintRuleMinMax(w, f, rule)
		case tagRequired:
			ruleNullableRequired(w, f)
		default:
			return errors.Wrap(errUnhandledTagRule(rule), "nullableInt")
		}
	}
	return nil
}

func nintRuleMinMax(w io.Writer, f structField, rule validationRule) {
	fmt.Fprintf(w, `
if val.%s.IsSet() && val.%s.Val %s %s {
	return fmt.Errorf("'%s': validation rule '%s(%s)' violated")
}
`[1:], f.Name(), f.Name(), ruleMinMaxOperator(rule.name), rule.value, jsonName(f), rule.name, rule.value)
}
