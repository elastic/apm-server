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

func (s *Service) Decode(input interface{}) error {
	if input == nil || s == nil {
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return errors.New("Invalid type for service")
	}
	df := utility.DataFetcher{}

	s.Name = df.String(raw, "name")
	s.Version = df.StringPtr(raw, "version")
	s.Environment = df.StringPtr(raw, "environment")
	s.Agent.Name = df.String(raw, "name", "agent")
	s.Agent.Version = df.String(raw, "version", "agent")
	s.Framework.Name = df.StringPtr(raw, "name", "framework")
	s.Framework.Version = df.StringPtr(raw, "version", "framework")
	s.Language.Name = df.StringPtr(raw, "name", "language")
	s.Language.Version = df.StringPtr(raw, "version", "language")
	s.Runtime.Name = df.StringPtr(raw, "name", "runtime")
	s.Runtime.Version = df.StringPtr(raw, "version", "runtime")
	return df.Err
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
