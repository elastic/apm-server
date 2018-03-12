package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pr "github.com/elastic/apm-server/processor"
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

func TestSystemEnrich(t *testing.T) {
	ip := "127.0.0.1"
	hostname := "host1"
	for _, test := range []struct {
		sys   *System
		input pr.Intake
		out   *System
	}{
		{
			sys:   nil,
			input: pr.Intake{},
			out:   nil,
		},
		{
			sys:   nil,
			input: pr.Intake{SystemIP: ip},
			out:   &System{IP: &ip},
		},
		{
			sys:   &System{Hostname: &hostname},
			input: pr.Intake{UserIP: ip},
			out:   &System{Hostname: &hostname},
		},
		{
			sys:   &System{Hostname: &hostname},
			input: pr.Intake{SystemIP: ip},
			out:   &System{Hostname: &hostname, IP: &ip},
		},
	} {
		assert.Equal(t, test.out, test.sys.Enrich(test.input))
	}
}
