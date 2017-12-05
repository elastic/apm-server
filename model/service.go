package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Service struct {
	Name         string
	Version      *string
	Pid          *int
	ProcessTitle *string `mapstructure:"process_title"`
	Environment  *string
	Argv         []string
	Language     Language
	Runtime      Runtime
	Framework    Framework
	Agent        Agent
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

type TransformService func(a *Service) common.MapStr

func (s *Service) MinimalTransform() common.MapStr {
	svc := common.MapStr{
		"name": s.Name,
		"agent": common.MapStr{
			"name":    s.Agent.Name,
			"version": s.Agent.Version,
		},
	}
	return svc
}

func (s *Service) Transform() common.MapStr {
	enhancer := utility.NewMapStrEnhancer()
	svc := s.MinimalTransform()
	enhancer.Add(svc, "version", s.Version)
	enhancer.Add(svc, "pid", s.Pid)
	enhancer.Add(svc, "process_title", s.ProcessTitle)
	enhancer.Add(svc, "environment", s.Environment)
	enhancer.Add(svc, "argv", s.Argv)

	lang := common.MapStr{}
	enhancer.Add(lang, "name", s.Language.Name)
	enhancer.Add(lang, "version", s.Language.Version)
	enhancer.Add(svc, "language", lang)

	runtime := common.MapStr{}
	enhancer.Add(runtime, "name", s.Runtime.Name)
	enhancer.Add(runtime, "version", s.Runtime.Version)
	enhancer.Add(svc, "runtime", runtime)

	framework := common.MapStr{}
	enhancer.Add(framework, "name", s.Framework.Name)
	enhancer.Add(framework, "version", s.Framework.Version)
	enhancer.Add(svc, "framework", framework)

	return svc
}
