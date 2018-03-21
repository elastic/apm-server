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

func TestProcessDecode(t *testing.T) {
	pid, ppid, title, argv := 123, 456, "foo", []string{"a", "b"}
	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		p           *Process
	}{
		{input: nil, err: nil, p: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), p: nil},
		{input: "", err: errors.New("Invalid type for process"), p: nil},
		{
			input: map[string]interface{}{"pid": "123"},
			err:   errors.New("Error fetching field"),
			p:     &Process{Ppid: nil, Title: nil, Argv: nil},
		},
		{
			input: map[string]interface{}{
				"pid": 123.0, "ppid": 456.0, "title": title, "argv": []interface{}{"a", "b"},
			},
			err: nil,
			p:   &Process{Pid: pid, Ppid: &ppid, Title: &title, Argv: argv},
		},
	} {
		proc, out := DecodeProcess(test.input, test.inpErr)
		assert.Equal(t, test.p, proc)
		assert.Equal(t, test.err, out)
	}
}
