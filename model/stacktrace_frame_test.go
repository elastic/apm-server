package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestStacktraceFrameTransform(t *testing.T) {
	filename := "some file"
	lineno := 12
	colno := 0
	path := "~/some/abs_path"
	context := "context"
	fct := "st function"
	module := "some_module"
	libraryFrame := true
	tests := []struct {
		StFrame StacktraceFrame
		Output  common.MapStr
	}{
		{
			StFrame: StacktraceFrame{Filename: filename, Lineno: lineno},
			Output:  common.MapStr{"filename": filename, "line": common.MapStr{"number": lineno}},
		},
		{
			StFrame: StacktraceFrame{
				AbsPath:      &path,
				Filename:     filename,
				Lineno:       lineno,
				Colno:        &colno,
				ContextLine:  &context,
				Module:       &module,
				Function:     &fct,
				LibraryFrame: &libraryFrame,
				Vars:         map[string]interface{}{"k1": "v1", "k2": "v2"},
				PreContext:   []string{"prec1", "prec2"},
				PostContext:  []string{"postc1", "postc2"},
			},
			Output: common.MapStr{
				"abs_path":      "~/some/abs_path",
				"filename":      "some file",
				"function":      "st function",
				"module":        "some_module",
				"library_frame": true,
				"vars":          common.MapStr{"k1": "v1", "k2": "v2"},
				"context": common.MapStr{
					"pre":  []string{"prec1", "prec2"},
					"post": []string{"postc1", "postc2"},
				},
				"line": common.MapStr{
					"number":  12,
					"column":  0,
					"context": "context",
				},
			},
		},
	}

	for _, test := range tests {
		output := (&test.StFrame).Transform()
		assert.Equal(t, test.Output, output)
	}
}
