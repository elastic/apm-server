package transaction

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/beats/libbeat/common"
)

func TestSpanTransform(t *testing.T) {
	transactionId := "123"
	emptyOut := common.MapStr{
		"duration":    common.MapStr{"us": 0},
		"name":        "",
		"start":       common.MapStr{"us": 0},
		"transaction": common.MapStr{"id": "123"},
		"type":        "",
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
				"type":        "",
				"start":       common.MapStr{"us": 0},
				"duration":    common.MapStr{"us": 0},
				"stacktrace":  []common.MapStr{{"filename": "", "line": common.MapStr{"number": 0}}},
				"transaction": common.MapStr{"id": "123"},
				"name":        "",
			},
			Msg: "Span with empty Stacktrace, default Stacktrace Transform",
		},
		{
			Span:   Span{},
			Output: emptyOut,
			Msg:    "Empty Span",
		},
		{
			Span: Span{
				Id:       &tid,
				Name:     "myspan",
				Type:     "myspantype",
				Start:    0.65,
				Duration: 1.20,
				StacktraceFrames: []m.StacktraceFrame{
					{
						AbsPath: &path,
					},
				},
				Context: common.MapStr{"key": "val"},
				Parent:  &parent,
			},
			Output: common.MapStr{
				"duration":    common.MapStr{"us": 1200},
				"id":          1,
				"name":        "myspan",
				"start":       common.MapStr{"us": 650},
				"transaction": common.MapStr{"id": "123"},
				"type":        "myspantype",
				"stacktrace":  []common.MapStr{{"abs_path": "test/path", "filename": "", "line": common.MapStr{"number": 0}}},
				"parent":      12,
			},
			Msg: "Full Span, transformFn for Stacktrace Transform",
		},
	}

	for idx, test := range tests {
		output := test.Span.Transform(transactionId, m.App{})
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
