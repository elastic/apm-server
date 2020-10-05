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
)

// tstruct implements generation logic for creating validation rules
// on struct types
type tstruct struct {
	generators map[string]vGenerators
}

type vGenerators interface {
	validation(io.Writer, []structField, structField) error
}

func newTstruct(pkg string) *tstruct {
	return &tstruct{
		generators: map[string]vGenerators{
			fmt.Sprintf("%s.String", pkg):    newNstring(),
			fmt.Sprintf("%s.Int", pkg):       newNint(),
			fmt.Sprintf("%s.Float64", pkg):   newNint(),
			fmt.Sprintf("%s.Interface", pkg): newNinterface(),
		}}
}

func (gen *tstruct) validation(w io.Writer, fields []structField, f structField, isCustomStruct bool) error {
	// if field is a custom struct, call its validation function
	if isCustomStruct {
		fmt.Fprintf(w, `
if err := val.%s.validate(); err != nil{
	return errors.Wrapf(err, "%s")
}
`[1:], f.Name(), jsonName(f))
	}

	// call defined validation generators
	if nGenerator, ok := gen.generators[f.Type().String()]; ok {
		return nGenerator.validation(w, fields, f)
	}
	// if no validation generator available for type
	// handle generally available rules ('required'),
	// and throw error for others
	vTag, err := validationTag(f.tag)
	if err != nil {
		return err
	}
	rules := validationRules(vTag)
	for _, rule := range rules {
		switch rule.name {
		case tagRequired:
			ruleNullableRequired(w, f)
		default:
			return fmt.Errorf("unhandled tag rule '%s'", rule)
		}
	}
	return nil
}
