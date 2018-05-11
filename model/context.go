package model

import (
	"errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type TransformContext struct {
	Service *Service
	Process *Process
	System  *System
	User    *User

	// cached transformed values
	service *common.MapStr
	process *common.MapStr
	system  *common.MapStr
	user    *common.MapStr
}

func (c *TransformContext) TransformInto(m common.MapStr) common.MapStr {
	if c.service == nil {
		service := c.Service.Transform()
		process := c.Process.Transform()
		system := c.System.Transform()
		user := c.User.Transform()

		c.service = &service
		c.process = &process
		c.system = &system
		c.user = &user
	}

	if m == nil {
		m = common.MapStr{}
	} else {
		for k, v := range m {
			// normalize map entries by calling utility.Add
			utility.Add(m, k, v)
		}
	}

	utility.Add(m, "service", *c.service)
	utility.Add(m, "process", *c.process)
	utility.Add(m, "system", *c.system)
	utility.MergeAdd(m, "user", *c.user)

	return m
}

func DecodeContext(input interface{}, err error) (*TransformContext, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for header")
	}

	tc := TransformContext{}
	tc.Process, err = DecodeProcess(raw["process"], err)
	tc.Service, err = DecodeService(raw["service"], err)
	tc.System, err = DecodeSystem(raw["system"], err)
	tc.User, err = DecodeUser(raw["user"], err)

	return &tc, err
}
