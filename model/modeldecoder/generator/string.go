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
)

func generateJSONPropertyString(info *fieldInfo, parent *property, child *property) error {
	child.Type.add(TypeNameString)
	parent.Properties[jsonSchemaName(info.field)] = child
	return setPropertyRulesString(info, child)
}

func setPropertyRulesString(info *fieldInfo, p *property) error {
	for tagName, tagValue := range info.tags {
		switch tagName {
		case tagEnum:
			enumVars, ok := info.parsed.enumVariables[tagValue]
			if !ok {
				return fmt.Errorf("unhandled %s tag value %s", tagName, tagValue)
			}
			for _, val := range enumVars {
				v := val
				p.Enum = append(p.Enum, &v)
			}
			// allow null value if field is not required
			if _, ok := info.tags[tagRequired]; !ok {
				p.Enum = append(p.Enum, nil)
			}
			delete(info.tags, tagName)
		case tagMaxLength:
			p.MaxLength = json.Number(tagValue)
			delete(info.tags, tagName)
		case tagMinLength:
			p.MinLength = json.Number(tagValue)
			delete(info.tags, tagName)
		case tagPattern:
			val, ok := info.parsed.patternVariables[tagValue]
			if !ok {
				return fmt.Errorf("unhandled %s tag value %s", tagName, tagValue)
			}
			p.Pattern = val
			delete(info.tags, tagName)
		}
	}
	return nil
}
