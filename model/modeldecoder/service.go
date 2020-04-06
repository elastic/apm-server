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

package modeldecoder

import (
	"github.com/elastic/apm-server/model/field"
	"github.com/elastic/apm-server/model/metadata"
)

func decodeService(input map[string]interface{}, hasShortFieldNames bool, out *metadata.Service) {
	if input == nil {
		return
	}
	fieldName := field.Mapper(hasShortFieldNames)
	decodeString(input, fieldName("name"), &out.Name)
	decodeString(input, fieldName("version"), &out.Version)
	decodeString(input, fieldName("environment"), &out.Environment)
	if node := getObject(input, fieldName("node")); node != nil {
		decodeString(node, fieldName("configured_name"), &out.Node.Name)
	}
	if agent := getObject(input, fieldName("agent")); agent != nil {
		decodeString(agent, fieldName("name"), &out.Agent.Name)
		decodeString(agent, fieldName("version"), &out.Agent.Version)
		decodeString(agent, fieldName("ephemeral_id"), &out.Agent.EphemeralId)
	}
	if framework := getObject(input, fieldName("framework")); framework != nil {
		decodeString(framework, fieldName("name"), &out.Framework.Name)
		decodeString(framework, fieldName("version"), &out.Framework.Version)
	}
	if language := getObject(input, fieldName("language")); language != nil {
		decodeString(language, fieldName("name"), &out.Language.Name)
		decodeString(language, fieldName("version"), &out.Language.Version)
	}
	if runtime := getObject(input, fieldName("runtime")); runtime != nil {
		decodeString(runtime, fieldName("name"), &out.Runtime.Name)
		decodeString(runtime, fieldName("version"), &out.Runtime.Version)
	}
}
