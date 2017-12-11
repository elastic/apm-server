package transaction

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/beats/libbeat/common"
)

func TestSpanTransform(t *testing.T) {
	path := "test/path"
	parent := 12
	tid := 1

	tests := []struct {
		Span   Span
		Output common.MapStr
		Msg    string
	}{
		{
			Span: Span{},
			Output: common.MapStr{
				"type":     "",
				"start":    common.MapStr{"us": 0},
				"duration": common.MapStr{"us": 0},
				"name":     "",
			},
			Msg: "Span without a Stacktrace",
		},
		{
			Span: Span{
				Id:       &tid,
				Name:     "myspan",
				Type:     "myspantype",
				Start:    0.65,
				Duration: 1.20,
				Stacktrace: []m.StacktraceFrame{
					{AbsPath: &path},
				},
				Context: common.MapStr{"key": "val"},
				Parent:  &parent,
			},
			Output: common.MapStr{
				"duration":   common.MapStr{"us": 1200},
				"id":         1,
				"name":       "myspan",
				"start":      common.MapStr{"us": 650},
				"type":       "myspantype",
				"stacktrace": []common.MapStr{{"abs_path": path, "filename": "", "line": common.MapStr{"number": 0}}},
				"parent":     12,
			},
			Msg: "Full Span",
		},
	}

	for idx, test := range tests {
		output := test.Span.Transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
