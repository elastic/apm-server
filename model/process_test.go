package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestProcessTransformDefinition(t *testing.T) {
	myfn := func(fn TransformProcess) string { return "ok" }
	res := myfn((*Process).Transform)
	assert.Equal(t, "ok", res)
}

func TestProcessTransform(t *testing.T) {
	pid := 1234
	processTitle := "node"
	argv := []string{
		"node",
		"server.js",
	}

	tests := []struct {
		Process Process
		Output  common.MapStr
	}{
		{
			Process: Process{},
			Output:  common.MapStr{"pid": 0},
		},
		{
			Process: Process{
				Pid:   pid,
				Title: &processTitle,
				Argv:  argv,
			},
			Output: common.MapStr{
				"pid":   pid,
				"title": processTitle,
				"argv":  argv,
			},
		},
	}

	for _, test := range tests {
		output := test.Process.Transform()
		assert.Equal(t, test.Output, output)
	}

}
