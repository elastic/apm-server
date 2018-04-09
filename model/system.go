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

func DecodeSystem(input interface{}, err error) (*System, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for system")
	}
	decoder := utility.ManualDecoder{}
	system := System{
		Hostname:     decoder.StringPtr(raw, "hostname"),
		Platform:     decoder.StringPtr(raw, "platform"),
		Architecture: decoder.StringPtr(raw, "architecture"),
		IP:           decoder.StringPtr(raw, "ip"),
	}
	return &system, decoder.Err
}

func (s *System) Transform() common.MapStr {
	if s == nil {
		return nil
	}
	system := common.MapStr{}
	utility.Add(system, "hostname", s.Hostname)
	utility.Add(system, "architecture", s.Architecture)
	utility.Add(system, "platform", s.Platform)
	if s.IP != nil && *s.IP != "" {
		utility.Add(system, "ip", s.IP)
	}
	return system
}
