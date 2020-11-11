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
)

func generateJSONPropertyInterface(info *fieldInfo, parent *property, child *property) error {
	name := jsonSchemaName(info.field)
	inputTypes, ok := info.tags[tagInputTypes]
	if ok {
		propTypes, err := propertyTypesFromTag(tagInputTypes, inputTypes)
		if err != nil {
			return err
		}
		for _, t := range propTypes {
			switch t {
			case TypeNameBool:
				if err := generateJSONPropertyBool(info, parent, child); err != nil {
					return err
				}
			case TypeNameString:
				if _, ok := info.tags[tagEnum]; ok && len(propTypes) > 1 {
					return fmt.Errorf("validation tag %s not allowed when multiple values defined for tag %s", tagEnum, tagInputTypes)
				}
				if err := generateJSONPropertyString(info, parent, child); err != nil {
					return err
				}
			case TypeNameInteger:
				if err := generateJSONPropertyInteger(info, parent, child); err != nil {
					return err
				}
			case TypeNameNumber:
				if err := generateJSONPropertyJSONNumber(info, parent, child); err != nil {
					return err
				}
			case TypeNameObject:
				child.Type.add(TypeNameObject)
			default:
				return fmt.Errorf("unhandled value %s for tag %s", t, tagInputTypes)
			}
		}
		child.Type.names = propTypes
		delete(info.tags, tagInputTypes)
	} else {
		// no type is specified for interface therefore all input types are allowed
		// set type to nil if property is not required, otherwise only reset type names
		if !child.Type.required {
			child.Type = nil
		} else {
			child.Type.names = nil
		}
	}
	// NOTE(simitt): targetTypes have never been reflected on schema
	delete(info.tags, tagTargetType)
	parent.Properties[name] = child
	return nil
}
