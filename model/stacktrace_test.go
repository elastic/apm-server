package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestStacktraceTransform(t *testing.T) {
	tr := true
	fa := false
	tests := []struct {
		Stacktrace Stacktrace
		Output     common.MapStr
		Msg        string
	}{
		{
			Stacktrace: Stacktrace{},
			Output:     nil,
			Msg:        "Stacktrace with no Frames",
		},
		{
			Stacktrace: Stacktrace{Frames: []StacktraceFrame{{}}},
			Output: common.MapStr{
				"frames":              []common.MapStr{{"filename": "", "line": common.MapStr{"number": 0}}},
				"only_library_frames": false,
			},
			Msg: "Stacktrace with empty Frame, unknown library frames",
		},
		{
			Stacktrace: Stacktrace{
				Frames: []StacktraceFrame{{LibraryFrame: &tr}},
			},
			Output: common.MapStr{
				"frames": []common.MapStr{
					{"filename": "", "line": common.MapStr{"number": 0}, "library_frame": true},
				},
				"only_library_frames": true,
			},
			Msg: "Stacktrace with only library frames",
		},
		{
			Stacktrace: Stacktrace{
				Frames: []StacktraceFrame{
					{Lineno: 1, LibraryFrame: &fa},
					{Lineno: 2, LibraryFrame: &fa},
					{Lineno: 3, LibraryFrame: &fa},
				},
			},
			Output: common.MapStr{
				"frames": []common.MapStr{
					{"filename": "", "line": common.MapStr{"number": 1}, "library_frame": false},
					{"filename": "", "line": common.MapStr{"number": 2}, "library_frame": false},
					{"filename": "", "line": common.MapStr{"number": 3}, "library_frame": false},
				},
				"only_library_frames": false,
			},
			Msg: "Stacktrace with no library frames",
		},
		{
			Stacktrace: Stacktrace{
				Frames: []StacktraceFrame{
					{Lineno: 1, LibraryFrame: &fa},
					{Lineno: 2, LibraryFrame: &tr},
					{Lineno: 3, LibraryFrame: &fa},
				},
			},
			Output: common.MapStr{
				"frames": []common.MapStr{
					{"filename": "", "line": common.MapStr{"number": 1}, "library_frame": false},
					{"filename": "", "line": common.MapStr{"number": 2}, "library_frame": true},
					{"filename": "", "line": common.MapStr{"number": 3}, "library_frame": false},
				},
				"only_library_frames": false,
			},
			Msg: "Stacktrace with mix of frames",
		},
	}

	for idx, test := range tests {
		output := test.Stacktrace.Transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
