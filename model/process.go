package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Process struct {
	Pid   int
	Ppid  *int
	Title *string
	Argv  []string
}

func (p *Process) Transform() common.MapStr {
	if p == nil {
		return nil
	}
	svc := common.MapStr{}
	utility.Add(svc, "pid", p.Pid)
	utility.Add(svc, "ppid", p.Ppid)
	utility.Add(svc, "title", p.Title)
	utility.Add(svc, "argv", p.Argv)

	return svc
}
