package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Context struct {
	service common.MapStr
	process common.MapStr
	system  common.MapStr
	user    common.MapStr
}

func NewContext(service *Service, process *Process, system *System, user *User) *Context {
	return &Context{
		service: service.Transform(),
		process: process.Transform(),
		system:  system.Transform(),
		user:    user.Transform(),
	}
}

func (c *Context) Transform(m common.MapStr) common.MapStr {
	if m == nil {
		m = common.MapStr{}
	} else {
		for k, v := range m {
			// normalize map entries by calling utility.Add
			utility.Add(m, k, v)
		}
	}
	utility.Add(m, "service", c.service)
	utility.Add(m, "process", c.process)
	utility.Add(m, "system", c.system)
	utility.MergeAdd(m, "user", c.user)
	return m
}
