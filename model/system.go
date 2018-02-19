package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type System struct {
	Hostname     *string
	Architecture *string
	Platform     *string
}

func (s *System) Transform() common.MapStr {
	if s == nil || (s.Hostname == nil && s.Architecture == nil && s.Platform == nil) {
		return nil
	}
	system := common.MapStr{}
	utility.AddStrPtr(system, "hostname", s.Hostname)
	utility.AddStrPtr(system, "architecture", s.Architecture)
	utility.AddStrPtr(system, "platform", s.Platform)

	return system
}
