package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestProcessTransform(t *testing.T) {
	var process *Process

	pid := 1234
	ppid := 45234
	processTitle := "node"
	argv := []string{"node", "server.js"}

	tests := []struct {
		Process *Process
		Output  common.MapStr
	}{
		{
			Process: process,
			Output:  nil,
		},
		{
			Process: &Process{},
			Output:  common.MapStr{},
		},
		{
			Process: &Process{
				Pid:   &pid,
				Ppid:  &ppid,
				Title: &processTitle,
				Argv:  argv,
			},
			Output: common.MapStr{
				"pid":   &pid,
				"ppid":  &ppid,
				"title": &processTitle,
				"argv":  argv,
			},
		},
	}

	for _, test := range tests {
		output := test.Process.Transform()
		assert.Equal(t, test.Output, output)
	}

}
