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
		Stacktrace        Stacktrace
		Output            []common.MapStr
		OnlyLibraryFrames bool
		Msg               string
	}{
		{
			Stacktrace:        Stacktrace{StacktraceFrame{}},
			Output:            []common.MapStr{{"filename": "", "line": common.MapStr{"number": 0}}},
			OnlyLibraryFrames: false,
			Msg:               "Stacktrace with empty Frame, unknown library frames",
		},
		{
			Stacktrace: Stacktrace{StacktraceFrame{LibraryFrame: &tr}},
			Output: []common.MapStr{
				{"filename": "", "line": common.MapStr{"number": 0}, "library_frame": true},
			},
			OnlyLibraryFrames: true,
			Msg:               "Stacktrace with only library frames",
		},
		{
			Stacktrace: Stacktrace{
				StacktraceFrame{Lineno: 1, LibraryFrame: &fa},
				StacktraceFrame{Lineno: 2, LibraryFrame: &fa},
				StacktraceFrame{Lineno: 3, LibraryFrame: &fa},
			},
			Output: []common.MapStr{
				{"filename": "", "line": common.MapStr{"number": 1}, "library_frame": false},
				{"filename": "", "line": common.MapStr{"number": 2}, "library_frame": false},
				{"filename": "", "line": common.MapStr{"number": 3}, "library_frame": false},
			},
			OnlyLibraryFrames: false,
			Msg:               "Stacktrace with no library frames",
		},
		{
			Stacktrace: Stacktrace{
				StacktraceFrame{Lineno: 1, LibraryFrame: &fa},
				StacktraceFrame{Lineno: 2, LibraryFrame: &tr},
				StacktraceFrame{Lineno: 3, LibraryFrame: &fa},
			},
			Output: []common.MapStr{
				{"filename": "", "line": common.MapStr{"number": 1}, "library_frame": false},
				{"filename": "", "line": common.MapStr{"number": 2}, "library_frame": true},
				{"filename": "", "line": common.MapStr{"number": 3}, "library_frame": false},
			},
			OnlyLibraryFrames: false,
			Msg:               "Stacktrace with mix of frames",
		},
	}

	for idx, test := range tests {
		output, onlyLib := test.Stacktrace.Transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
		assert.Equal(t, test.OnlyLibraryFrames, onlyLib, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
