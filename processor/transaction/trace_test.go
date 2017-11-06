package transaction

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/beats/libbeat/common"
)

func TestTraceTransform(t *testing.T) {
	nilFn := func(s *m.Stacktrace) []common.MapStr {
		return nil
	}
	emptyFn := func(s *m.Stacktrace) []common.MapStr {
		return []common.MapStr{}
	}
	transformFn := func(s *m.Stacktrace) []common.MapStr {
		return []common.MapStr{{"foo": "bar"}}
	}
	transactionId := "123"
	emptyOut := common.MapStr{
		"duration":       common.MapStr{"us": 0},
		"name":           "",
		"start":          common.MapStr{"us": 0},
		"transaction_id": "123",
		"type":           "",
	}

	path := "test/path"
	parent := 12
	tid := 1
	frames := []m.StacktraceFrame{{}}

	tests := []struct {
		Trace  Trace
		Output common.MapStr
		Msg    string
	}{
		{
			Trace: Trace{StacktraceFrames: frames},
			Output: common.MapStr{
				"type":           "",
				"start":          common.MapStr{"us": 0},
				"duration":       common.MapStr{"us": 0},
				"stacktrace":     []common.MapStr{{"filename": "", "line": common.MapStr{"number": 0}}},
				"transaction_id": "123",
				"name":           "",
			},
			Msg: "Trace with empty Stacktrace, default Stacktrace Transform",
		},
		{
			Trace:  Trace{TransformStacktrace: emptyFn},
			Output: emptyOut,
			Msg:    "Empty Trace, emptyFn for Stacktrace Transform",
		},
		{
			Trace:  Trace{TransformStacktrace: nilFn},
			Output: emptyOut,
			Msg:    "Empty Trace, nilFn for Stacktrace Transform",
		},
		{
			Trace: Trace{
				Id:       &tid,
				Name:     "mytrace",
				Type:     "mytracetype",
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
				"duration":       common.MapStr{"us": 1200},
				"id":             1,
				"name":           "mytrace",
				"start":          common.MapStr{"us": 650},
				"transaction_id": "123",
				"type":           "mytracetype",
				"stacktrace":     []common.MapStr{{"foo": "bar"}},
				"parent":         12,
			},
			Msg: "Full Trace, transformFn for Stacktrace Transform",
		},
	}

	for idx, test := range tests {
		output := test.Trace.Transform(transactionId)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
