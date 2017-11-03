package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type System struct {
	Hostname     *string `json:"hostname"`
	Architecture *string `json:"architecture"`
	Platform     *string `json:"platform"`
}

func (s *System) Transform() common.MapStr {
	if s == nil {
		return nil
	}
	enhancer := utility.NewMapStrEnhancer()
	system := common.MapStr{}
	enhancer.Add(system, "hostname", s.Hostname)
	enhancer.Add(system, "architecture", s.Architecture)
	enhancer.Add(system, "platform", s.Platform)

	return system
}
