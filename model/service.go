package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Service struct {
	Name        *string
	Version     *string
	Environment *string
	Agent       struct {
		Name    *string
		Version *string
	}
	Language struct {
		Name    *string
		Version *string
	}
	Runtime struct {
		Name    *string
		Version *string
	}
	Framework struct {
		Name    *string
		Version *string
	}
}

func (s *Service) MinimalTransform() common.MapStr {
	if s == nil {
		return nil
	}
	svc := common.MapStr{}
	utility.AddStrPtr(svc, "name", s.Name)
	ag := common.MapStr{}
	utility.AddStrPtr(ag, "name", s.Agent.Name)
	utility.AddStrPtr(ag, "version", s.Agent.Version)
	utility.AddCommonMapStr(svc, "agent", ag)
	if len(svc) == 0 {
		return nil
	}
	return svc
}

func (s *Service) Transform() common.MapStr {
	if s == nil {
		return nil
	}
	svc := s.MinimalTransform()
	utility.AddStrPtr(svc, "version", s.Version)
	utility.AddStrPtr(svc, "environment", s.Environment)

	m := common.MapStr{}
	utility.AddStrPtr(m, "name", s.Language.Name)
	utility.AddStrPtr(m, "version", s.Language.Version)
	utility.AddCommonMapStr(svc, "language", m)

	m = common.MapStr{}
	utility.AddStrPtr(m, "name", s.Runtime.Name)
	utility.AddStrPtr(m, "version", s.Runtime.Version)
	utility.AddCommonMapStr(svc, "runtime", m)

	m = common.MapStr{}
	utility.AddStrPtr(m, "name", s.Framework.Name)
	utility.AddStrPtr(m, "version", s.Framework.Version)
	utility.AddCommonMapStr(svc, "framework", m)
	if len(svc) == 0 {
		return nil
	}
	return svc
}
