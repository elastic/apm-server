package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Process struct {
	Pid   int
	Title *string
	Argv  []string
}

type TransformProcess func(a *Process) common.MapStr

func (p *Process) Transform() common.MapStr {
	if p == nil {
		return nil
	}
	enhancer := utility.NewMapStrEnhancer()
	svc := common.MapStr{}
	enhancer.Add(svc, "pid", p.Pid)
	enhancer.Add(svc, "title", p.Title)
	enhancer.Add(svc, "argv", p.Argv)

	return svc
}
