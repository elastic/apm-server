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
	for _, test := range []struct {
		input interface{}
		err   error
		s     *System
	}{
		{input: nil, err: nil, s: &System{}},
		{input: "", err: errors.New("Invalid type for system"), s: &System{}},
		{
			input: map[string]interface{}{"hostname": 1},
			err:   errors.New("Invalid type for field"),
			s:     &System{},
		},
		{
			input: map[string]interface{}{
				"hostname": &host, "architecture": arch, "platform": &platform, "ip": &ip,
			},
			err: nil,
			s:   &System{Hostname: &host, Architecture: &arch, Platform: &platform, IP: &ip},
		},
	} {
		sys := &System{}
		out := sys.Decode(test.input)
		assert.Equal(t, test.s, sys)
		assert.Equal(t, test.err, out)
	}

	var s *System
	assert.Nil(t, s.Decode("a"), nil)
}
