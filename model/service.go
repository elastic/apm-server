package model

import (
	"errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Service struct {
	Name        string
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
	Name    string
	Version string
}

func DecodeService(input interface{}, err error) (*Service, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for service")
	}
	decoder := utility.ManualDecoder{}
	service := Service{
		Name:        decoder.String(raw, "name"),
		Version:     decoder.StringPtr(raw, "version"),
		Environment: decoder.StringPtr(raw, "environment"),
		Agent: Agent{
			Name:    decoder.String(raw, "name", "agent"),
			Version: decoder.String(raw, "version", "agent"),
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

func (s *Service) MinimalTransform() common.MapStr {
	if s == nil {
		return nil
	}
	svc := common.MapStr{"name": s.Name}
	agent := common.MapStr{}
	utility.Add(agent, "name", s.Agent.Name)
	utility.Add(agent, "version", s.Agent.Version)
	utility.Add(svc, "agent", agent)
	return svc
}

func (s *Service) Transform() common.MapStr {
	if s == nil {
		return nil
	}
	svc := s.MinimalTransform()
	utility.Add(svc, "version", s.Version)
	utility.Add(svc, "environment", s.Environment)

	lang := common.MapStr{}
	utility.Add(lang, "name", s.Language.Name)
	utility.Add(lang, "version", s.Language.Version)
	utility.Add(svc, "language", lang)

	runtime := common.MapStr{}
	utility.Add(runtime, "name", s.Runtime.Name)
	utility.Add(runtime, "version", s.Runtime.Version)
	utility.Add(svc, "runtime", runtime)

	framework := common.MapStr{}
	utility.Add(framework, "name", s.Framework.Name)
	utility.Add(framework, "version", s.Framework.Version)
	utility.Add(svc, "framework", framework)

	return svc
}
