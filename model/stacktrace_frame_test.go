package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
)

func TestStacktraceFrameTransform(t *testing.T) {
	filename := "some file"
	lineno := 1
	colno := 55
	path := "~/./some/abs_path"
	context := "context"
	fct := "some function"
	module := "some_module"
	libraryFrame := true
	serviceVersion := "1.0"
	tests := []struct {
		StFrame StacktraceFrame
		Output  common.MapStr
		Service Service
		Msg     string
	}{
		{
			StFrame: StacktraceFrame{Filename: filename, Lineno: lineno},
			Output:  common.MapStr{"filename": filename, "line": common.MapStr{"number": lineno}},
			Service: Service{Name: "myService"},
			Msg:     "Minimal StacktraceFrame",
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
				"abs_path":      "~/./some/abs_path",
				"filename":      "some file",
				"function":      "some function",
				"module":        "some_module",
				"library_frame": true,
				"vars":          common.MapStr{"k1": "v1", "k2": "v2"},
				"context": common.MapStr{
					"pre":  []string{"prec1", "prec2"},
					"post": []string{"postc1", "postc2"},
				},
				"line": common.MapStr{
					"number":  1,
					"column":  55,
					"context": "context",
				},
			},
			Service: Service{Name: "myService", Version: &serviceVersion},
			Msg:     "Full StacktraceFrame",
		},
	}

	for idx, test := range tests {
		output := (&test.StFrame).Transform(&pr.Config{}, test.Service)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestApplySourcemap(t *testing.T) {
	colno := 1
	fct := "original function"
	absPath := "original absPath"
	tests := []struct {
		fr  StacktraceFrame
		out common.MapStr
		msg string
	}{
		{
			fr: StacktraceFrame{},
			out: common.MapStr{
				"filename": "", "line": common.MapStr{"number": 0},
				"sourcemap": common.MapStr{
					"error":   "Colno mandatory for sourcemapping.",
					"updated": false,
				},
			},
			msg: "No colno",
		},
		{
			fr: StacktraceFrame{Colno: &colno, Lineno: 9},
			out: common.MapStr{
				"filename": "", "line": common.MapStr{"column": 1, "number": 9},
				"sourcemap": common.MapStr{
					"error":   "Some untyped error",
					"updated": false,
				},
			},
			msg: "Some error occured in mapper.",
		},
		{
			fr: StacktraceFrame{Colno: &colno, Lineno: 8},
			out: common.MapStr{
				"filename": "", "line": common.MapStr{"column": 1, "number": 8},
				"sourcemap": common.MapStr{
					"updated": false,
				},
			},
			msg: "Some access error occured in mapper.",
		},
		{
			fr: StacktraceFrame{Colno: &colno, Lineno: 7},
			out: common.MapStr{
				"filename": "", "line": common.MapStr{"column": 1, "number": 7},
				"sourcemap": common.MapStr{
					"updated": false,
					"error":   "Some mapping error",
				},
			},
			msg: "Some mapping error occured in mapper.",
		},
		{
			fr: StacktraceFrame{Colno: &colno, Lineno: 6},
			out: common.MapStr{
				"filename": "", "line": common.MapStr{"column": 1, "number": 6},
				"sourcemap": common.MapStr{
					"updated": false,
					"error":   "Some key error",
				},
			},
			msg: "Some key error occured in mapper.",
		},
		{
			fr: StacktraceFrame{
				Colno:    &colno,
				Lineno:   5,
				Filename: "original filename",
				Function: &fct,
				AbsPath:  &absPath,
			},
			out: common.MapStr{
				"filename": "original filename",
				"function": "original function",
				"line":     common.MapStr{"column": 100, "number": 500},
				"abs_path": "changed path",
				"sourcemap": common.MapStr{
					"updated": true,
				},
			},
			msg: "Sourcemap with empty filename and function mapping applied.",
		},
		{
			fr: StacktraceFrame{
				Colno:    &colno,
				Lineno:   4,
				Filename: "original filename",
				Function: &fct,
				AbsPath:  &absPath,
			},
			out: common.MapStr{
				"filename": "changed filename",
				"function": "changed function",
				"line":     common.MapStr{"column": 100, "number": 400},
				"abs_path": "changed path",
				"sourcemap": common.MapStr{
					"updated": true,
				},
			},
			msg: "Full sourcemap mapping applied.",
		},
	}

	ver := "1"
	service := Service{Name: "foo", Version: &ver}
	for idx, test := range tests {
		output := (&test.fr).Transform(&pr.Config{SmapMapper: &FakeMapper{}}, service)
		assert.Equal(t, test.out, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.msg))
	}
}

func TestBuildSourcemap(t *testing.T) {
	version := "1.0"
	path := "././a/b/../c"
	tests := []struct {
		service Service
		fr      StacktraceFrame
		out     string
	}{
		{service: Service{}, fr: StacktraceFrame{}, out: ""},
		{service: Service{Version: &version}, fr: StacktraceFrame{}, out: "1.0"},
		{service: Service{Name: "foo"}, fr: StacktraceFrame{}, out: "foo"},
		{service: Service{}, fr: StacktraceFrame{AbsPath: &path}, out: "a/c"},
		{
			service: Service{Name: "foo", Version: &version},
			fr:      StacktraceFrame{AbsPath: &path},
			out:     "foo_1.0_a/c",
		},
	}
	for _, test := range tests {
		id := test.fr.buildSourcemapId(test.service)
		assert.Equal(t, test.out, (&id).Key())
	}
}

// Fake implemenations for Mapper

type FakeMapper struct{}

func (m *FakeMapper) Apply(smapId sourcemap.Id, lineno, colno int) (*sourcemap.Mapping, error) {
	switch lineno {
	case 9:
		return nil, errors.New("Some untyped error")
	case 8:
		return nil, sourcemap.Error{Kind: sourcemap.AccessError}
	case 7:
		return nil, sourcemap.Error{Kind: sourcemap.MapError, Msg: "Some mapping error"}
	case 6:
		return nil, sourcemap.Error{Kind: sourcemap.KeyError, Msg: "Some key error"}
	case 5:
		return &sourcemap.Mapping{
			Filename: "",
			Function: "",
			Colno:    100,
			Lineno:   500,
			Path:     "changed path",
		}, nil
	default:
		return &sourcemap.Mapping{
			Filename: "changed filename",
			Function: "changed function",
			Colno:    100,
			Lineno:   400,
			Path:     "changed path",
		}, nil
	}
}
func (m *FakeMapper) NewSourcemapAdded(smapId sourcemap.Id) {}
