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

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

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

	metadata := Metadata{}
	metadata.Service, err = DecodeService(raw["service"], err)
	metadata.System, err = DecodeSystem(raw["system"], err)
	metadata.Process, err = DecodeProcess(raw["process"], err)
	metadata.User, err = DecodeUser(raw["user"], err)

	if err != nil {
		return nil, err
	}

	return &metadata, err
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

func (m *Metadata) Merge(eventContext common.MapStr) common.MapStr {
	eventContext = m.normalizeContext(eventContext)

	utility.Add(eventContext, "system", m.System.Transform())
	utility.Add(eventContext, "process", m.Process.Transform())
	utility.MergeAdd(eventContext, "user", m.User.Transform())
	utility.MergeAdd(eventContext, "service", m.Service.Transform())

	return eventContext
}

func (m *Metadata) MergeMinimal(eventContext common.MapStr) common.MapStr {
	eventContext = m.normalizeContext(eventContext)

	utility.MergeAdd(eventContext, "service", m.Service.MinimalTransform())
	return eventContext
}
