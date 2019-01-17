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

package metadata

import (
	"errors"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/model/metadata/generated/schema"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/common"
)

var cachedModelSchema = validation.CreateSchema(schema.ModelSchema, "metadata")

func ModelSchema() *jsonschema.Schema {
	return cachedModelSchema
}

type Metadata struct {
	Service *Service
	Process *Process
	System  *System
	User    *User
}

func DecodeMetadata(input interface{}) (*Metadata, error) {
	if input == nil {
		return nil, nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for metadata")
	}

	var err error
	var service *Service
	var system *System
	var process *Process
	var user *User
	service, err = DecodeService(raw["service"], err)
	system, err = DecodeSystem(raw["system"], err)
	process, err = DecodeProcess(raw["process"], err)
	user, err = DecodeUser(raw["user"], err)

	if err != nil {
		return nil, err
	}
	return NewMetadata(service, system, process, user), nil
}

func NewMetadata(service *Service, system *System, process *Process, user *User) *Metadata {
	return &Metadata{
		Service: service,
		System:  system,
		Process: process,
		User:    user,
	}
}

func (m *Metadata) Merge(fields common.MapStr) common.MapStr {
	utility.MergeAdd(fields, "agent", m.Service.agentFields())
	utility.Add(fields, "host", m.System.fields())
	utility.Add(fields, "process", m.Process.fields())
	utility.MergeAdd(fields, "service", m.Service.fields())
	utility.AddIfNil(fields, "user", m.User.Fields())
	utility.AddIfNil(fields, "client", m.User.ClientFields())
	utility.AddIfNil(fields, "user_agent", m.User.UserAgentFields())
	utility.Add(fields, "container", m.System.containerFields())
	utility.Add(fields, "kubernetes", m.System.kubernetesFields())
	return fields
}

func (m *Metadata) MergeMinimal(fields common.MapStr) common.MapStr {
	utility.MergeAdd(fields, "agent", m.Service.agentFields())
	utility.MergeAdd(fields, "service", m.Service.minimalFields())
	return fields
}
