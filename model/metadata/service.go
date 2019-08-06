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

type Service struct {
	Name        *string
	Version     *string
	Environment *string
	Language    Language
	Runtime     Runtime
	Framework   Framework
	Agent       Agent
}

type Language struct {
	Name    *string
	Version *string
}
type Runtime struct {
	Name    *string
	Version *string
}
type Framework struct {
	Name    *string
	Version *string
}
type Agent struct {
	Name        *string
	Version     *string
	EphemeralId *string
}

func DecodeService(input interface{}, err error) (*Service, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for service")
	}
	decoder := utility.ManualDecoder{}
	service := Service{
		Name:        decoder.StringPtr(raw, "name"),
		Version:     decoder.StringPtr(raw, "version"),
		Environment: decoder.StringPtr(raw, "environment"),
		Agent: Agent{
			Name:        decoder.StringPtr(raw, "name", "agent"),
			Version:     decoder.StringPtr(raw, "version", "agent"),
			EphemeralId: decoder.StringPtr(raw, "ephemeral_id", "agent"),
		},
		Framework: Framework{
			Name:    decoder.StringPtr(raw, "name", "framework"),
			Version: decoder.StringPtr(raw, "version", "framework"),
		},
		Language: Language{
			Name:    decoder.StringPtr(raw, "name", "language"),
			Version: decoder.StringPtr(raw, "version", "language"),
		},
		Runtime: Runtime{
			Name:    decoder.StringPtr(raw, "name", "runtime"),
			Version: decoder.StringPtr(raw, "version", "runtime"),
		},
	}
	return &service, decoder.Err
}

func (s *Service) MinimalFields() common.MapStr {
	if s == nil {
		return nil
	}
	svc := common.MapStr{}
	utility.Set(svc, "name", s.Name)
	utility.Set(svc, "environment", s.Environment)
	return svc
}

func (s *Service) Fields() common.MapStr {
	if s == nil {
		return nil
	}
	svc := s.MinimalFields()
	utility.Set(svc, "version", s.Version)

	lang := common.MapStr{}
	utility.Set(lang, "name", s.Language.Name)
	utility.Set(lang, "version", s.Language.Version)
	utility.Set(svc, "language", lang)

	runtime := common.MapStr{}
	utility.Set(runtime, "name", s.Runtime.Name)
	utility.Set(runtime, "version", s.Runtime.Version)
	utility.Set(svc, "runtime", runtime)

	framework := common.MapStr{}
	utility.Set(framework, "name", s.Framework.Name)
	utility.Set(framework, "version", s.Framework.Version)
	utility.Set(svc, "framework", framework)

	return svc
}

func (s *Service) AgentFields() common.MapStr {
	if s == nil {
		return nil
	}
	return s.Agent.fields()
}

func (a *Agent) fields() common.MapStr {
	if a == nil {
		return nil
	}
	agent := common.MapStr{}
	utility.Set(agent, "name", a.Name)
	utility.Set(agent, "version", a.Version)
	utility.Set(agent, "ephemeral_id", a.EphemeralId)
	return agent
}
