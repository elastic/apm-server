package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestSystemTransform(t *testing.T) {
	var system *System
	architecture := "x64"
	hostname := "a.b.com"
	platform := "darwin"

	tests := []struct {
		System *System
		Output common.MapStr
	}{
		{
			System: system,
			Output: nil,
		},
		{
			System: &System{},
			Output: nil,
		},
		{
			System: &System{
				Architecture: &architecture,
				Hostname:     &hostname,
				Platform:     &platform,
			},
			Output: common.MapStr{
				"hostname":     &hostname,
				"architecture": &architecture,
				"platform":     &platform,
			},
		},
	}

	for _, test := range tests {
		output := test.System.Transform()
		assert.Equal(t, test.Output, output)
	}
}
