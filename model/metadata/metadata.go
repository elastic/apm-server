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
	Service    *Service
	Process    *Process
	System     *System
	User       *User
	Kubernetes *Kubernetes
	Container  *Container

	serviceFields common.MapStr
	processFields common.MapStr
	systemFields  common.MapStr
	userFields    common.MapStr

	kubernetesFields common.MapStr
	containerFields  common.MapStr

	minimalServiceFields common.MapStr
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
	var kubernetes *Kubernetes
	var container *Container
	var process *Process
	var user *User

	service, err = DecodeService(raw["service"], err)

	system, err = DecodeSystem(raw["system"], err)

	if s, ok := raw["system"]; ok {
		if smap, ok := s.(map[string]interface{}); ok {
			kubernetes, err = DecodeKubernetes(smap["kubernetes"], err)
			container, err = DecodeContainer(smap["container"], err)
		}
	}

	process, err = DecodeProcess(raw["process"], err)
	user, err = DecodeUser(raw["user"], err)

	if err != nil {
		return nil, err
	}
	return NewMetadata(service, system, process, user, kubernetes, container), nil
}

func NewMetadata(service *Service, system *System, process *Process, user *User, kubernetes *Kubernetes, container *Container) *Metadata {
	m := Metadata{
		Service:    service,
		System:     system,
		Process:    process,
		User:       user,
		Kubernetes: kubernetes,
		Container:  container,
	}

	m.serviceFields = m.Service.fields()
	m.systemFields = m.System.fields()
	m.processFields = m.Process.fields()
	m.userFields = m.User.fields()
	m.kubernetesFields = m.Kubernetes.fields()
	m.containerFields = m.Container.fields()

	m.minimalServiceFields = m.Service.minimalFields()
	return &m
}

func (m *Metadata) normalizeContext(eventContext common.MapStr) common.MapStr {
	if eventContext == nil {
		return common.MapStr{}
	} else {
		for k, v := range eventContext {
			// normalize map entries by calling utility.Add
			utility.Add(eventContext, k, v)
		}
		return eventContext
	}
}

func (m *Metadata) Merge(event common.MapStr) common.MapStr {
	eventContext, ok := event["context"].(common.MapStr)
	if ok {
		eventContext = m.normalizeContext(eventContext)

		utility.Add(eventContext, "system", m.systemFields)
		utility.Add(eventContext, "process", m.processFields)
		utility.MergeAdd(eventContext, "user", m.userFields)
		utility.MergeAdd(eventContext, "service", m.serviceFields)
		event["context"] = eventContext
	}

	// add these at the root
	utility.Add(event, "kubernetes", m.kubernetesFields)
	utility.Add(event, "docker", common.MapStr{"container": m.containerFields})

	return event
}

func (m *Metadata) MergeMinimal(eventContext common.MapStr) common.MapStr {
	eventContext = m.normalizeContext(eventContext)

	utility.MergeAdd(eventContext, "service", m.minimalServiceFields)
	return eventContext
}
