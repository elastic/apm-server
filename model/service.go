package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Service struct {
	Name        string    `json:"name"`
	Version     *string   `json:"version"`
	Environment *string   `json:"environment"`
	Language    Language  `json:"language"`
	Runtime     Runtime   `json:"runtime"`
	Framework   Framework `json:"framework"`
	Agent       Agent     `json:"agent"`
}

type Language struct {
	Name    *string `json:"name"`
	Version *string `json:"version"`
}
type Runtime struct {
	Name    *string `json:"name"`
	Version *string `json:"version"`
}
type Framework struct {
	Name    *string `json:"name"`
	Version *string `json:"version"`
}
type Agent struct {
	Name    string `json:"name"`
	Version string `json:"version"`
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
