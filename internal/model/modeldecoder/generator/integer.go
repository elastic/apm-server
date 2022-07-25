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
)

func generateJSONPropertyInteger(info *fieldInfo, parent *property, child *property) error {
	child.Type.add(TypeNameInteger)
	parent.Properties[jsonSchemaName(info.field)] = child
	return setPropertyRulesInteger(info, child)
}

func setPropertyRulesInteger(info *fieldInfo, p *property) error {
	for tagName, tagValue := range info.tags {
		switch tagName {
		case tagMax:
			p.Max = json.Number(tagValue)
			delete(info.tags, tagName)
		case tagMin:
			p.Min = json.Number(tagValue)
			delete(info.tags, tagName)
		}
	}
	return nil
}
