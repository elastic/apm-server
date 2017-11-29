package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestStacktraceTransform(t *testing.T) {
	tests := []struct {
		Stacktrace []StacktraceFrame
		Output     []common.MapStr
		Msg        string
	}{
		{
			Stacktrace: []StacktraceFrame{{}},
			Output:     []common.MapStr{{"filename": "", "line": common.MapStr{"number": 0}}},
			Msg:        "Stacktrace with empty Frame, default Frame Transformation",
		},
		{
			Stacktrace: []StacktraceFrame{},
			Output:     []common.MapStr(nil),
			Msg:        "Stacktrace without frames",
		},
		{
			Stacktrace: []StacktraceFrame{{Filename: "foo"}},
			Output:     []common.MapStr{{"filename": "foo", "line": common.MapStr{"number": 0}}},
			Msg:        "Stacktrace with Frame, Frame Transformation set to transform Function",
		},
	}

	for idx, test := range tests {
		output := TransformStacktrace(test.Stacktrace, App{})
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
