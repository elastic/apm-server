package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type System struct {
	Hostname     *string
	Architecture *string
	Platform     *string
	IP           *string
}

func (s *System) Transform() common.MapStr {
	if s == nil {
		return nil
	}
	system := common.MapStr{}
	utility.Add(system, "hostname", s.Hostname)
	utility.Add(system, "architecture", s.Architecture)
	utility.Add(system, "platform", s.Platform)
	utility.Add(system, "ip", s.IP)

	return system
}
