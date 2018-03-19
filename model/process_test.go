package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

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
			Output:  common.MapStr{},
		},
		{
			Process: Process{
				Pid:   &pid,
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

func TestProcessDecode(t *testing.T) {
	pid, ppid, title, argv := 123, 456, "foo", []string{"a", "b"}
	for _, test := range []struct {
		input interface{}
		err   error
		p     *Process
	}{
		{input: nil, err: nil, p: &Process{}},
		{input: "", err: errors.New("Invalid type for process"), p: &Process{}},
		{
			input: map[string]interface{}{"pid": "123"},
			err:   errors.New("Invalid type for field"),
			p:     &Process{},
		},
		{
			input: map[string]interface{}{
				"pid": &pid, "ppid": &ppid, "title": &title, "argv": argv,
			},
			err: nil,
			p:   &Process{Pid: &pid, Ppid: &ppid, Title: &title, Argv: argv},
		},
	} {
		proc := &Process{}
		out := proc.Decode(test.input)
		assert.Equal(t, test.p, proc)
		assert.Equal(t, test.err, out)
	}

	var p *Process
	assert.Nil(t, p.Decode("a"), nil)
}
