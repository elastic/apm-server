package model

import (
	"errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Process struct {
	Pid   int
	Ppid  *int
	Title *string
	Argv  []string
}

func DecodeProcess(input interface{}, err error) (*Process, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for process")
	}
	df := utility.DataFetcher{}
	process := Process{
		Ppid:  df.IntPtr(raw, "ppid"),
		Title: df.StringPtr(raw, "title"),
		Argv:  df.StringArr(raw, "argv"),
	}
	if pid := df.IntPtr(raw, "pid"); pid != nil {
		process.Pid = *pid
	}
	return &process, df.Err
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
