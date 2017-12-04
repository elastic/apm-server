package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Service struct {
	Name         string    `json:"name"`
	Version      *string   `json:"version"`
	Pid          *int      `json:"pid"`
	ProcessTitle *string   `json:"process_title"`
	Environment  *string   `json:"environment"`
	Argv         []string  `json:"argv"`
	Language     Language  `json:"language"`
	Runtime      Runtime   `json:"runtime"`
	Framework    Framework `json:"framework"`
	Agent        Agent     `json:"agent"`
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
