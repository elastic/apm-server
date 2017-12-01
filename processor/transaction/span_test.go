package transaction

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/beats/libbeat/common"
)

func TestSpanTransform(t *testing.T) {
	nilFn := func(s *m.Stacktrace) []common.MapStr {
		return nil
	}
	emptyFn := func(s *m.Stacktrace) []common.MapStr {
		return []common.MapStr{}
	}
	transformFn := func(s *m.Stacktrace) []common.MapStr {
		return []common.MapStr{{"foo": "bar"}}
	}
	emptyOut := common.MapStr{
		"duration": common.MapStr{"us": 0},
		"name":     "",
		"start":    common.MapStr{"us": 0},
		"type":     "",
	}

	path := "test/path"
	parent := 12
	tid := 1
	frames := []m.StacktraceFrame{{}}

	tests := []struct {
		Span   Span
		Output common.MapStr
		Msg    string
	}{
		{
			Span: Span{StacktraceFrames: frames},
			Output: common.MapStr{
				"type":       "",
				"start":      common.MapStr{"us": 0},
				"duration":   common.MapStr{"us": 0},
				"stacktrace": []common.MapStr{{"filename": "", "line": common.MapStr{"number": 0}}},
				"name":       "",
			},
			Msg: "Span with empty Stacktrace, default Stacktrace Transform",
		},
		{
			Span:   Span{TransformStacktrace: emptyFn},
			Output: emptyOut,
			Msg:    "Empty Span, emptyFn for Stacktrace Transform",
		},
		{
			Span:   Span{TransformStacktrace: nilFn},
			Output: emptyOut,
			Msg:    "Empty Span, nilFn for Stacktrace Transform",
		},
		{
			Span: Span{
				Id:       &tid,
				Name:     "myspan",
				Type:     "myspantype",
				Start:    0.65,
				Duration: 1.20,
				StacktraceFrames: m.StacktraceFrames{
					{
						AbsPath: &path,
					},
				},
				Context:             common.MapStr{"key": "val"},
				Parent:              &parent,
				TransformStacktrace: transformFn,
			},
			Output: common.MapStr{
				"duration":   common.MapStr{"us": 1200},
				"id":         1,
				"name":       "myspan",
				"start":      common.MapStr{"us": 650},
				"type":       "myspantype",
				"stacktrace": []common.MapStr{{"foo": "bar"}},
				"parent":     12,
			},
			Msg: "Full Span, transformFn for Stacktrace Transform",
		},
	}

	for idx, test := range tests {
		output := test.Span.Transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
