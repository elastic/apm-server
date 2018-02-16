package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Service struct {
	Name        string
	Version     *string
	Environment *string
	Language
	Runtime
	Framework
	Agent
}

type Language struct {
	LanguageName    *string
	LanguageVersion *string
}
type Runtime struct {
	RuntimeName    *string
	RuntimeVersion *string
}
type Framework struct {
	FrameworkName    *string
	FrameworkVersion *string
}
type Agent struct {
	AgentName    string
	AgentVersion string
}

type TransformService func(a *Service) common.MapStr

func (s *Service) MinimalTransform() common.MapStr {
	svc := common.MapStr{
		"name": s.Name,
		"agent": common.MapStr{
			"name":    s.Agent.AgentName,
			"version": s.Agent.AgentVersion,
		},
	}
	return svc
}

func (s *Service) Transform() common.MapStr {
	svc := s.MinimalTransform()
	utility.Add(svc, "version", s.Version)
	utility.Add(svc, "environment", s.Environment)

	lang := common.MapStr{}
	utility.Add(lang, "name", s.Language.LanguageName)
	utility.Add(lang, "version", s.Language.LanguageVersion)
	utility.Add(svc, "language", lang)

	runtime := common.MapStr{}
	utility.Add(runtime, "name", s.Runtime.RuntimeName)
	utility.Add(runtime, "version", s.Runtime.RuntimeVersion)
	utility.Add(svc, "runtime", runtime)

	framework := common.MapStr{}
	utility.Add(framework, "name", s.Framework.FrameworkName)
	utility.Add(framework, "version", s.Framework.FrameworkVersion)
	utility.Add(svc, "framework", framework)

	return svc
}
