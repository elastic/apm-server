package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/common"
)

func TestStacktraceTransform(t *testing.T) {
	sName := "n"
	service := Service{Name: &sName}
	colno, l1, l4, l5, l6, l8 := 1, 1, 4, 5, 6, 8
	fName, origFname := "/webpack", "original filename"
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
			Output:     []common.MapStr{{"exclude_from_grouping": new(bool)}},
			Msg:        "Stacktrace with empty Frame",
		},
		{
			Stacktrace: Stacktrace{
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l4,
					Filename: &origFname,
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{Colno: &colno, Lineno: &l6, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{Colno: &colno, Lineno: &l8, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l5,
					Filename: &origFname,
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l4,
					Filename: &fName,
					AbsPath:  &absPath,
				},
			},
			Output: []common.MapStr{
				{
					"abs_path": &absPath, "filename": &origFname, "function": &fct,
					"line":                  common.MapStr{"column": &l1, "number": &l4},
					"exclude_from_grouping": new(bool),
				},
				{
					"abs_path": &absPath, "function": &fct,
					"line":                  common.MapStr{"column": &l1, "number": &l6},
					"exclude_from_grouping": new(bool),
				},
				{
					"abs_path": &absPath, "function": &fct,
					"line":                  common.MapStr{"column": &l1, "number": &l8},
					"exclude_from_grouping": new(bool),
				},
				{
					"abs_path": &absPath, "filename": &origFname, "function": &fct,
					"line":                  common.MapStr{"column": &l1, "number": &l5},
					"exclude_from_grouping": new(bool),
				},
				{
					"abs_path": &absPath, "filename": &fName,
					"line":                  common.MapStr{"column": &l1, "number": &l4},
					"exclude_from_grouping": new(bool),
				},
			},
			Msg: "Stacktrace with sourcemapping",
		},
	}

	for idx, test := range tests {
		output := test.Stacktrace.Transform(&pr.Config{}, service)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestStacktraceTransformWithSourcemapping(t *testing.T) {
	sName := "n"
	service := Service{Name: &sName}
	colno, l1, l4, l5, l6, l8, l100, l400, l500 := 1, 1, 4, 5, 6, 8, 100, 400, 500
	fName, origFname, changedFname := "/webpack", "original filename", "changed filename"
	fct, origFct, changedFct := "original function", "original function", "changed function"
	unknownFct, anonymFct := "<unknown>", "<anonymous>"
	absPath, origPath, changedPath := "original path", "original path", "changed path"
	truthy := true
	keyErr := "Some key error"
	smapErr := "Colno and Lineno mandatory for sourcemapping."

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
					"exclude_from_grouping": new(bool),
					"sourcemap": common.MapStr{
						"error":   &smapErr,
						"updated": new(bool),
					},
				},
			},
			Msg: "Stacktrace with empty Frame",
		},
		{
			Stacktrace: Stacktrace{
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l4,
					Filename: &origFname,
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{Colno: &colno, Lineno: &l6, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{Colno: &colno, Lineno: &l8, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l5,
					Filename: &origFname,
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l4,
					Filename: &fName,
					AbsPath:  &absPath,
				},
			},
			Output: []common.MapStr{
				{
					"abs_path": &changedPath, "filename": &changedFname, "function": &unknownFct,
					"line":                  common.MapStr{"column": &l100, "number": &l400},
					"exclude_from_grouping": new(bool),
					"sourcemap":             common.MapStr{"updated": &truthy},
					"original": common.MapStr{
						"abs_path": &origPath,
						"colno":    &l1,
						"filename": &origFname,
						"function": &origFct,
						"lineno":   &l4,
					},
				},
				{
					"abs_path": &origPath, "function": &origFct,
					"line":                  common.MapStr{"column": &l1, "number": &l6},
					"exclude_from_grouping": new(bool),
					"sourcemap":             common.MapStr{"updated": new(bool), "error": &keyErr},
				},
				{
					"abs_path": &origPath, "function": &origFct,
					"line":                  common.MapStr{"column": &l1, "number": &l8},
					"exclude_from_grouping": new(bool),
				},
				{
					"abs_path": &changedPath, "filename": &origFname, "function": &changedFct,
					"line":                  common.MapStr{"column": &l100, "number": &l500},
					"exclude_from_grouping": new(bool),
					"sourcemap":             common.MapStr{"updated": &truthy},
					"original": common.MapStr{
						"abs_path": &origPath,
						"colno":    &l1,
						"filename": &origFname,
						"function": &origFct,
						"lineno":   &l5,
					},
				},
				{
					"abs_path": &changedPath, "filename": &changedFname, "function": &anonymFct,
					"line":                  common.MapStr{"column": &l100, "number": &l400},
					"exclude_from_grouping": new(bool),
					"sourcemap":             common.MapStr{"updated": &truthy},
					"original": common.MapStr{
						"abs_path": &origPath,
						"colno":    &l1,
						"filename": &fName,
						"lineno":   &l4,
					},
				},
			},
			Msg: "Stacktrace with sourcemapping",
		},
	}

	for idx, test := range tests {
		// run `Stacktrace.Transform` twice to ensure method is idempotent
		test.Stacktrace.Transform(&pr.Config{SmapMapper: &FakeMapper{}}, service)
		output := test.Stacktrace.Transform(&pr.Config{SmapMapper: &FakeMapper{}}, service)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
