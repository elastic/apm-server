package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/beats/libbeat/common"
)

func TestStacktraceDecode(t *testing.T) {
	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		s           *Stacktrace
	}{
		{input: nil, err: nil, s: nil},
		{input: nil, inpErr: errors.New("msg"), err: errors.New("msg"), s: nil},
		{input: "", err: errors.New("Invalid type for stacktrace"), s: nil},
		{
			input: []interface{}{"foo"},
			err:   errors.New("Invalid type for stacktrace frame"),
			s:     &Stacktrace{nil},
		},
		{
			input: []interface{}{map[string]interface{}{
				"filename": "file", "lineno": 1.0},
			},
			err: nil,
			s: &Stacktrace{
				&StacktraceFrame{Filename: "file", Lineno: 1},
			},
		},
	} {
		s, err := DecodeStacktrace(test.input, test.inpErr)
		assert.Equal(t, test.s, s)
		assert.Equal(t, test.err, err)
	}
}

func TestStacktraceTransform(t *testing.T) {
	service := Service{Name: "myService"}
	colno := 1
	fct := "original function"
	absPath := "original path"

	tests := []struct {
		Stacktrace Stacktrace
		Output     []common.MapStr
		Msg        string
	}{
		{
			Stacktrace: Stacktrace{},
			Output:     nil,
			Msg:        "Empty Stacktrace",
		},
		{
			Stacktrace: Stacktrace{&StacktraceFrame{}},
			Output: []common.MapStr{
				{
					"filename":              "",
					"line":                  common.MapStr{"number": 0},
					"exclude_from_grouping": false,
				},
			},
			Msg: "Stacktrace with empty Frame",
		},
		{
			Stacktrace: Stacktrace{
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   4,
					Filename: "original filename",
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{Colno: &colno, Lineno: 6, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{Colno: &colno, Lineno: 8, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   5,
					Filename: "original filename",
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   4,
					Filename: "/webpack",
					AbsPath:  &absPath,
				},
			},
			Output: []common.MapStr{
				{
					"abs_path": "original path", "filename": "original filename", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 4},
					"exclude_from_grouping": false,
				},
				{
					"abs_path": "original path", "filename": "", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 6},
					"exclude_from_grouping": false,
				},
				{
					"abs_path": "original path", "filename": "", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 8},
					"exclude_from_grouping": false,
				},
				{
					"abs_path": "original path", "filename": "original filename", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 5},
					"exclude_from_grouping": false,
				},
				{
					"abs_path": "original path", "filename": "/webpack",
					"line":                  common.MapStr{"column": 1, "number": 4},
					"exclude_from_grouping": false,
				},
			},
			Msg: "Stacktrace with sourcemapping",
		},
	}

	for idx, test := range tests {
		output := test.Stacktrace.Transform(config.Config{}, service)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestStacktraceTransformWithSourcemapping(t *testing.T) {
	service := Service{Name: "myService"}
	colno := 1
	fct := "original function"
	absPath := "original path"

	tests := []struct {
		Stacktrace Stacktrace
		Output     []common.MapStr
		Msg        string
	}{
		{
			Stacktrace: Stacktrace{},
			Output:     nil,
			Msg:        "Empty Stacktrace",
		},
		{
			Stacktrace: Stacktrace{&StacktraceFrame{}},
			Output: []common.MapStr{
				{"filename": "",
					"line":                  common.MapStr{"number": 0},
					"exclude_from_grouping": false,
					"sourcemap": common.MapStr{
						"error":   "Colno mandatory for sourcemapping.",
						"updated": false,
					},
				},
			},
			Msg: "Stacktrace with empty Frame",
		},
		{
			Stacktrace: Stacktrace{
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   4,
					Filename: "original filename",
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{Colno: &colno, Lineno: 6, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{Colno: &colno, Lineno: 8, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   5,
					Filename: "original filename",
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   4,
					Filename: "/webpack",
					AbsPath:  &absPath,
				},
			},
			Output: []common.MapStr{
				{
					"abs_path": "changed path", "filename": "changed filename", "function": "<unknown>",
					"line":                  common.MapStr{"column": 100, "number": 400},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": true},
					"original": common.MapStr{
						"abs_path": "original path",
						"colno":    1,
						"filename": "original filename",
						"function": "original function",
						"lineno":   4,
					},
				},
				{
					"abs_path": "original path", "filename": "", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 6},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": false, "error": "Some key error"},
				},
				{
					"abs_path": "original path", "filename": "", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 8},
					"exclude_from_grouping": false,
				},
				{
					"abs_path": "changed path", "filename": "original filename", "function": "changed function",
					"line":                  common.MapStr{"column": 100, "number": 500},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": true},
					"original": common.MapStr{
						"abs_path": "original path",
						"colno":    1,
						"filename": "original filename",
						"function": "original function",
						"lineno":   5,
					},
				},
				{
					"abs_path": "changed path", "filename": "changed filename", "function": "<anonymous>",
					"line":                  common.MapStr{"column": 100, "number": 400},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": true},
					"original": common.MapStr{
						"abs_path": "original path",
						"colno":    1,
						"filename": "/webpack",
						"lineno":   4,
					},
				},
			},
			Msg: "Stacktrace with sourcemapping",
		},
	}

	for idx, test := range tests {
		// run `Stacktrace.Transform` twice to ensure method is idempotent
		test.Stacktrace.Transform(config.Config{SmapMapper: &FakeMapper{}}, service)
		output := test.Stacktrace.Transform(config.Config{SmapMapper: &FakeMapper{}}, service)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
