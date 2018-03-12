package model

import (
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type System struct {
	Hostname     *string `json:"hostname"`
	Architecture *string `json:"architecture"`
	Platform     *string `json:"platform"`
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

func (s *System) Enrich(input pr.Intake) *System {
	if input.SystemIP == "" {
		return s
	}
	if s == nil {
		s = &System{}
	}
	s.IP = &input.SystemIP
	return s
}
