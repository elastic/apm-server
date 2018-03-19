package model

import (
	"errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type System struct {
	Hostname     *string
	Architecture *string
	Platform     *string
	IP           *string
}

func (s *System) Decode(input interface{}) error {
	if input == nil || s == nil {
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return errors.New("Invalid type for system")
	}
	df := utility.DataFetcher{}
	s.Hostname = df.StringPtr(raw, "hostname")
	s.Platform = df.StringPtr(raw, "platform")
	s.Architecture = df.StringPtr(raw, "architecture")
	s.IP = df.StringPtr(raw, "ip")
	return df.Err
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
