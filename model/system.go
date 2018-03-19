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
	df := utility.DataFetcher{}
	system := System{
		Hostname:     df.StringPtr(raw, "hostname"),
		Platform:     df.StringPtr(raw, "platform"),
		Architecture: df.StringPtr(raw, "architecture"),
		IP:           df.StringPtr(raw, "ip"),
	}
	return &system, df.Err
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
