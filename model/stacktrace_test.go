package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestStacktraceTransformDefinition(t *testing.T) {
	myfn := func(fn TransformStacktrace) string { return "ok" }
	res := myfn((*Stacktrace).Transform)
	assert.Equal(t, "ok", res)
}

func TestStacktraceTransform(t *testing.T) {
	emptyFn := func(s *StacktraceFrame) common.MapStr {
		return nil
	}
	transformFn := func(s *StacktraceFrame) common.MapStr {
		return common.MapStr{"foo": "bar"}
	}
	frames := StacktraceFrames{StacktraceFrame{}}

	tests := []struct {
		Stacktrace Stacktrace
		Output     []common.MapStr
		Msg        string
	}{
		{
			Stacktrace: Stacktrace{Frames: frames},
			Output:     []common.MapStr{{"filename": "", "line": common.MapStr{"number": 0}}},
			Msg:        "Stacktrace with empty Frame, default Frame Transformation",
		},
		{
			Stacktrace: Stacktrace{TransformFrame: emptyFn, Frames: frames},
			Output:     []common.MapStr{common.MapStr(nil)},
			Msg:        "Stacktrace with Frame, Frame Transformation set to nil Function",
		},
		{
			Stacktrace: Stacktrace{TransformFrame: transformFn, Frames: frames},
			Output:     []common.MapStr{{"foo": "bar"}},
			Msg:        "Stacktrace with Frame, Frame Transformation set to transform Function",
		},
	}

	for idx, test := range tests {
		output := test.Stacktrace.Transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
