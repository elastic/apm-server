package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type NilSmap struct{}

func (s *NilSmap) Fetch(smapId utility.SmapID) (*sourcemap.Consumer, error) {
	return nil, nil
}
func (s *NilSmap) RemoveFromCache(smapId utility.SmapID) {}

type ErrorSmap struct{}

func (s *ErrorSmap) Fetch(smapId utility.SmapID) (*sourcemap.Consumer, error) {
	return nil, errors.New("Error when fetching sourcemap.")
}
func (s *ErrorSmap) RemoveFromCache(smapId utility.SmapID) {}

type ValidSmap struct{}

func (s *ValidSmap) Fetch(smapId utility.SmapID) (*sourcemap.Consumer, error) {
	fileBytes, err := tests.LoadDataAsBytes("data/valid/sourcemap/bundle.js.map")
	if err != nil {
		panic(err)
	}
	return sourcemap.Parse("", fileBytes)
}
func (s *ValidSmap) RemoveFromCache(smapId utility.SmapID) {}

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
	nilSmap := &NilSmap{}
	errorSmap := &ErrorSmap{}
	validSmap := &ValidSmap{}
	tests := []struct {
		StFrame      StacktraceFrame
		Output       common.MapStr
		Service      Service
		SmapAccessor utility.SmapAccessor
		Msg          string
	}{
		{
			StFrame:      StacktraceFrame{Filename: filename, Lineno: lineno},
			Output:       common.MapStr{"filename": filename, "line": common.MapStr{"number": lineno}},
			SmapAccessor: nil,
			Service:      Service{Name: "myService"},
			Msg:          "Minimal StacktraceFrame, no sourcemap mapping",
		},
		{
			StFrame: StacktraceFrame{
				Filename: filename,
				Lineno:   lineno,
				Colno:    &colno,
				AbsPath:  &path,
			},
			Output: common.MapStr{
				"filename": filename,
				"line":     common.MapStr{"number": lineno, "column": colno},
				"sourcemap": common.MapStr{
					"error":   "AbsPath, Colno, Service Name and Version mandatory for sourcemapping.",
					"updated": false,
				},
				"abs_path": path,
			},
			SmapAccessor: validSmap,
			Service:      Service{Name: "myService"},
			Msg:          "Minimal StacktraceFrame, no service version",
		},
		{
			StFrame: StacktraceFrame{
				Filename: filename,
				Lineno:   lineno,
				Colno:    &colno,
				AbsPath:  &path,
			},
			Output: common.MapStr{
				"filename": filename,
				"line":     common.MapStr{"number": lineno, "column": colno},
				"sourcemap": common.MapStr{
					"error":   "AbsPath, Colno, Service Name and Version mandatory for sourcemapping.",
					"updated": false,
				},
				"abs_path": path,
			},
			SmapAccessor: validSmap,
			Service:      Service{Version: &serviceVersion},
			Msg:          "Minimal StacktraceFrame, no service name",
		},
		{
			StFrame: StacktraceFrame{
				Filename: filename,
				Lineno:   lineno,
				Colno:    &colno,
			},
			Output: common.MapStr{
				"filename": filename,
				"line":     common.MapStr{"number": lineno, "column": colno},
				"sourcemap": common.MapStr{
					"error":   "AbsPath, Colno, Service Name and Version mandatory for sourcemapping.",
					"updated": false,
				},
			},
			SmapAccessor: validSmap,
			Service:      Service{Name: "myService", Version: &serviceVersion},
			Msg:          "Minimal StacktraceFrame, no abs path",
		},
		{
			StFrame: StacktraceFrame{
				Filename: filename,
				Lineno:   lineno,
				AbsPath:  &path,
			},
			Output: common.MapStr{
				"filename": filename,
				"line":     common.MapStr{"number": lineno},
				"sourcemap": common.MapStr{
					"error":   "AbsPath, Colno, Service Name and Version mandatory for sourcemapping.",
					"updated": false,
				},
				"abs_path": path,
			},
			SmapAccessor: validSmap,
			Service:      Service{Name: "myService", Version: &serviceVersion},
			Msg:          "Minimal StacktraceFrame, no colno",
		},
		{
			StFrame: StacktraceFrame{
				Filename: filename,
				Lineno:   lineno,
				Colno:    &colno,
				AbsPath:  &path,
			},
			Output: common.MapStr{
				"filename": filename,
				"line":     common.MapStr{"number": lineno, "column": colno},
				"abs_path": path,
				"sourcemap": common.MapStr{
					"error":   "No Sourcemap found for this StacktraceFrame.",
					"updated": false,
				},
			},
			SmapAccessor: nilSmap,
			Service:      Service{Name: "myService", Version: &serviceVersion},
			Msg:          "Minimal StacktraceFrame, no sourcemap.",
		},
		{
			StFrame: StacktraceFrame{
				Filename: filename,
				Lineno:   10000,
				Colno:    &colno,
				AbsPath:  &path,
			},
			Output: common.MapStr{
				"filename": filename,
				"line":     common.MapStr{"number": 10000, "column": colno},
				"abs_path": path,
				"sourcemap": common.MapStr{
					"error":   "No mapping for Lineno 10000 and Colno 55.",
					"updated": false,
				},
			},
			SmapAccessor: validSmap,
			Service:      Service{Name: "myService", Version: &serviceVersion},
			Msg:          "Minimal StacktraceFrame, no sourcemap mapping for lineno and colno.",
		},
		{
			StFrame: StacktraceFrame{
				AbsPath:  &path,
				Filename: filename,
				Lineno:   lineno,
				Colno:    &colno,
			},
			Output: common.MapStr{
				"abs_path": path,
				"filename": filename,
				"line": common.MapStr{
					"number": 1,
					"column": 55,
				},
				"sourcemap": common.MapStr{
					"error":   "Error when fetching sourcemap.",
					"updated": false,
				},
			},
			SmapAccessor: errorSmap,
			Service:      Service{Name: "myService", Version: &serviceVersion},
			Msg:          "Error in Sourcemap mapping",
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
				"filename":      "webpack:///webpack/bootstrap 6002740481c9666b0d38",
				"function":      "some function",
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
				"sourcemap": common.MapStr{
					"updated": true,
				},
			},
			SmapAccessor: validSmap,
			Service:      Service{Name: "myService", Version: &serviceVersion},
			Msg:          "Full StacktraceFrame",
		},
	}

	for idx, test := range tests {
		output := (&test.StFrame).Transform(test.Service, test.SmapAccessor)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
