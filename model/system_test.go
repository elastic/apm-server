package model

import (
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
			},
			Output: common.MapStr{
				"hostname":     hostname,
				"architecture": architecture,
				"platform":     platform,
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
		{
			System: System{
				Architecture: &architecture,
				Hostname:     &hostname,
				IP:           &ip,
			},
			Output: common.MapStr{
				"hostname":     hostname,
				"architecture": architecture,
				"ip":           ip,
			},
		},
	}

	for _, test := range tests {
		output := test.System.Transform()
		assert.Equal(t, test.Output, output)
	}
}
