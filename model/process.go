package model

import (
	"errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Process struct {
	Pid   *int
	Ppid  *int
	Title *string
	Argv  []string
}

func (p *Process) Decode(input interface{}) error {
	if input == nil || p == nil {
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return errors.New("Invalid type for process")
	}
	df := utility.DataFetcher{}
	p.Pid = df.IntPtr(raw, "pid")
	p.Ppid = df.IntPtr(raw, "ppid")
	p.Title = df.StringPtr(raw, "title")
	p.Argv = df.StringArr(raw, "argv")
	return df.Err
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
