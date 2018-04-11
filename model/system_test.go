package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestSystemTransform(t *testing.T) {

	architecture := "x64"
	hostname := "a.b.com"
	platform := "darwin"
	ip := "127.0.0.1"
	empty := ""

	tests := []struct {
		System System
		Output common.MapStr
	}{
		{
			System: System{},
			Output: common.MapStr{},
		},
		{
			System: System{
				IP: &empty,
			},
			Output: common.MapStr{},
		},
		{
			System: System{
				Architecture: &architecture,
				Hostname:     &hostname,
				Platform:     &platform,
				IP:           &ip,
			},
			Output: common.MapStr{
				"hostname":     hostname,
				"architecture": architecture,
				"platform":     platform,
				"ip":           ip,
			},
		},
		{
			System: System{
				Architecture: &architecture,
				Hostname:     &hostname,
			},
			Output: common.MapStr{
				"hostname":     hostname,
				"architecture": architecture,
			},
		},
	}

	for _, test := range tests {
		output := test.System.Transform()
		assert.Equal(t, test.Output, output)
	}
}

func TestSystemDecode(t *testing.T) {
	host, arch, platform, ip := "host", "amd", "osx", "127.0.0.1"
	inpErr := errors.New("some error")
	for _, test := range []struct {
		input         interface{}
		inputErr, err error
		s             *System
	}{
		{input: nil, err: nil, s: nil},
		{input: nil, inputErr: inpErr, err: inpErr, s: nil},
		{input: "", err: errors.New("Invalid type for system"), s: nil},
		{
			input: map[string]interface{}{"hostname": 1},
			err:   errors.New("Error fetching field"),
			s:     &System{Hostname: nil, Architecture: nil, Platform: nil, IP: nil},
		},
		{
			input: map[string]interface{}{
				"hostname": host, "architecture": arch, "platform": platform, "ip": ip,
			},
			err: nil,
			s:   &System{Hostname: &host, Architecture: &arch, Platform: &platform, IP: &ip},
		},
	} {
		sys, err := DecodeSystem(test.input, test.inputErr)
		assert.Equal(t, test.s, sys)
		assert.Equal(t, test.err, err)
	}
}
