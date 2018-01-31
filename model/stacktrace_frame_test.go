package model

import (
	"errors"
	"fmt"
	"regexp"
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
	tests := []struct {
		StFrame StacktraceFrame
		Output  common.MapStr
		Msg     string
	}{
		{
			StFrame: StacktraceFrame{Filename: filename, Lineno: lineno},
			Output: common.MapStr{
				"filename":              filename,
				"line":                  common.MapStr{"number": lineno},
				"exclude_from_grouping": false,
			},
			Msg: "Minimal StacktraceFrame",
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
				"exclude_from_grouping": false,
			},
			Msg: "Full StacktraceFrame",
		},
	}

	for idx, test := range tests {
		output := (&test.StFrame).Transform(&pr.Config{})
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestApplySourcemap(t *testing.T) {
	colno := 1
	fct := "original function"
	absPath := "original path"
	tests := []struct {
		fr                          StacktraceFrame
		lineno, colno               int
		filename, function, absPath string
		smapUpdated                 bool
		smapError                   string
		fct                         string
		outFct                      string
		msg                         string
	}{
		{
			fr:          StacktraceFrame{Lineno: 0, Function: &fct, AbsPath: &absPath},
			lineno:      0,
			filename:    "",
			function:    "original function",
			absPath:     "original path",
			smapUpdated: false,
			smapError:   "Colno mandatory for sourcemapping.",
			fct:         "<anonymous>",
			outFct:      "<anonymous>",
			msg:         "No colno",
		},
		{
			fr: StacktraceFrame{
				Colno:    &colno,
				Lineno:   9,
				Filename: "filename",
				Function: &fct,
				AbsPath:  &absPath,
			},
			colno:       1,
			lineno:      9,
			filename:    "filename",
			function:    "original function",
			absPath:     "original path",
			smapUpdated: false,
			smapError:   "Some untyped error",
			fct:         "<anonymous>",
			outFct:      "<anonymous>",
			msg:         "Some error occured in mapper.",
		},
		{
			fr:       StacktraceFrame{Colno: &colno, Lineno: 8, Function: &fct, AbsPath: &absPath},
			colno:    1,
			lineno:   8,
			filename: "",
			function: "original function",
			absPath:  "original path",
			fct:      "<anonymous>",
			outFct:   "<anonymous>",
			msg:      "Some access error occured in mapper.",
		},
		{
			fr:          StacktraceFrame{Colno: &colno, Lineno: 7, Function: &fct, AbsPath: &absPath},
			colno:       1,
			lineno:      7,
			filename:    "",
			function:    "original function",
			absPath:     "original path",
			smapUpdated: false,
			smapError:   "Some mapping error",
			fct:         "<anonymous>",
			outFct:      "<anonymous>",
			msg:         "Some mapping error occured in mapper.",
		},
		{
			fr:          StacktraceFrame{Colno: &colno, Lineno: 6, Function: &fct, AbsPath: &absPath},
			colno:       1,
			lineno:      6,
			filename:    "",
			function:    "original function",
			absPath:     "original path",
			smapUpdated: false,
			smapError:   "Some key error",
			fct:         "<anonymous>",
			outFct:      "<anonymous>",
			msg:         "Some key error occured in mapper.",
		},
		{
			fr: StacktraceFrame{
				Colno:    &colno,
				Lineno:   5,
				Filename: "original filename",
				Function: &fct,
				AbsPath:  &absPath,
			},
			colno:       100,
			lineno:      500,
			filename:    "original filename",
			function:    "other function",
			absPath:     "changed path",
			smapUpdated: true,

			fct:    "other function",
			outFct: "<unknown>",
			msg:    "Sourcemap with empty filename and function mapping applied.",
		},
		{
			fr: StacktraceFrame{
				Colno:    &colno,
				Lineno:   4,
				Filename: "original filename",
				Function: &fct,
				AbsPath:  &absPath,
			},
			colno:       100,
			lineno:      400,
			filename:    "changed filename",
			function:    "prev function",
			absPath:     "changed path",
			smapUpdated: true,

			fct:    "prev function",
			outFct: "changed function",
			msg:    "Full sourcemap mapping applied.",
		},
	}

	ver := "1"
	service := Service{Name: "foo", Version: &ver}
	for idx, test := range tests {
		output := (&test.fr).applySourcemap(&FakeMapper{}, service, test.fct)
		assert.Equal(t, test.outFct, output)
		assert.Equal(t, test.lineno, test.fr.Lineno, fmt.Sprintf("Failed at idx %v; %s", idx, test.msg))
		assert.Equal(t, test.filename, test.fr.Filename, fmt.Sprintf("Failed at idx %v; %s", idx, test.msg))
		assert.Equal(t, test.function, *test.fr.Function, fmt.Sprintf("Failed at idx %v; %s", idx, test.msg))
		assert.Equal(t, test.absPath, *test.fr.AbsPath, fmt.Sprintf("Failed at idx %v; %s", idx, test.msg))
		if test.colno != 0 {
			assert.Equal(t, test.colno, *test.fr.Colno, fmt.Sprintf("Failed at idx %v; %s", idx, test.msg))
		}
		if test.smapError != "" || test.smapUpdated {
			assert.Equal(t, test.smapUpdated, *test.fr.Sourcemap.Updated, fmt.Sprintf("Failed at idx %v; %s", idx, test.msg))
		}
		if test.smapError != "" {
			assert.Equal(t, test.smapError, *test.fr.Sourcemap.Error, fmt.Sprintf("Failed at idx %v; %s", idx, test.msg))
		}
	}
}

func TestIsLibraryFrame(t *testing.T) {
	assert.False(t, (&StacktraceFrame{}).IsLibraryFrame())
	assert.False(t, (&StacktraceFrame{LibraryFrame: new(bool)}).IsLibraryFrame())
	libFrame := true
	assert.True(t, (&StacktraceFrame{LibraryFrame: &libFrame}).IsLibraryFrame())
}

func TestIsSourcemapApplied(t *testing.T) {
	assert.False(t, (&StacktraceFrame{}).IsSourcemapApplied())

	fr := StacktraceFrame{Sourcemap: Sourcemap{Updated: new(bool)}}
	assert.False(t, fr.IsSourcemapApplied())

	libFrame := true
	fr = StacktraceFrame{Sourcemap: Sourcemap{Updated: &libFrame}}
	assert.True(t, fr.IsSourcemapApplied())
}

func TestExcludeFromGroupingKey(t *testing.T) {
	tests := []struct {
		fr      StacktraceFrame
		pattern string
		exclude bool
	}{
		{
			fr:      StacktraceFrame{},
			pattern: "",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: "/webpack"},
			pattern: "",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: "/webpack"},
			pattern: "/webpack/tmp",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: ""},
			pattern: "^/webpack",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: "/webpack"},
			pattern: "^/webpack",
			exclude: true,
		},
		{
			fr:      StacktraceFrame{Filename: "/webpack/test/e2e/general-usecase/app.e2e-bundle.js"},
			pattern: "^/webpack",
			exclude: true,
		},
		{
			fr:      StacktraceFrame{Filename: "/filename"},
			pattern: "^/webpack",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: "/filename/a"},
			pattern: "^/webpack",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: "webpack"},
			pattern: "^/webpack",
			exclude: false,
		},
	}

	for idx, test := range tests {
		var excludePattern *regexp.Regexp
		if test.pattern != "" {
			excludePattern = regexp.MustCompile(test.pattern)
		}
		out := test.fr.Transform(&pr.Config{ExcludeFromGrouping: excludePattern})
		exclude := out["exclude_from_grouping"]
		assert.Equal(t, test.exclude, exclude,
			fmt.Sprintf("(%v): Pattern: %v, Filename: %v, expected to be excluded: %v", idx, test.pattern, test.fr.Filename, test.exclude))
	}
}

func TestLibraryFrame(t *testing.T) {
	truthy := true
	falsy := false
	path := "/~/a/b"
	tests := []struct {
		fr           StacktraceFrame
		conf         *pr.Config
		libraryFrame *bool
		msg          string
	}{
		{fr: StacktraceFrame{},
			conf:         &pr.Config{},
			libraryFrame: nil,
			msg:          "Empty StacktraceFrame, empty config"},
		{fr: StacktraceFrame{AbsPath: &path},
			conf:         &pr.Config{LibraryPattern: nil},
			libraryFrame: nil,
			msg:          "No pattern"},
		{fr: StacktraceFrame{AbsPath: &path},
			conf:         &pr.Config{LibraryPattern: regexp.MustCompile("")},
			libraryFrame: &truthy,
			msg:          "Empty pattern"},
		{fr: StacktraceFrame{},
			conf:         &pr.Config{LibraryPattern: regexp.MustCompile("~")},
			libraryFrame: &falsy,
			msg:          "Empty StacktraceFrame"},
		{fr: StacktraceFrame{AbsPath: &path},
			conf:         &pr.Config{LibraryPattern: regexp.MustCompile("^~/")},
			libraryFrame: &falsy,
			msg:          "AbsPath given, no Match"},
		{fr: StacktraceFrame{Filename: "myFile.js"},
			conf:         &pr.Config{LibraryPattern: regexp.MustCompile("^~/")},
			libraryFrame: &falsy,
			msg:          "Filename given, no Match"},
		{fr: StacktraceFrame{AbsPath: &path, Filename: "myFile.js"},
			conf:         &pr.Config{LibraryPattern: regexp.MustCompile("^~/")},
			libraryFrame: &falsy,
			msg:          "AbsPath and Filename given, no Match"},
		{fr: StacktraceFrame{Filename: "/tmp"},
			conf:         &pr.Config{LibraryPattern: regexp.MustCompile("/tmp")},
			libraryFrame: &truthy,
			msg:          "Filename matching"},
		{fr: StacktraceFrame{AbsPath: &path},
			conf:         &pr.Config{LibraryPattern: regexp.MustCompile("~/")},
			libraryFrame: &truthy,
			msg:          "AbsPath matching"},
		{fr: StacktraceFrame{AbsPath: &path, Filename: "/a/b/c"},
			conf:         &pr.Config{LibraryPattern: regexp.MustCompile("~/")},
			libraryFrame: &truthy,
			msg:          "AbsPath matching, Filename not matching"},
		{fr: StacktraceFrame{AbsPath: &path, Filename: "/a/b/c"},
			conf:         &pr.Config{LibraryPattern: regexp.MustCompile("/a/b/c")},
			libraryFrame: &truthy,
			msg:          "AbsPath not matching, Filename matching"},
		{fr: StacktraceFrame{AbsPath: &path, Filename: "~/a/b/c"},
			conf:         &pr.Config{LibraryPattern: regexp.MustCompile("~/")},
			libraryFrame: &truthy,
			msg:          "AbsPath and Filename matching"},
	}

	for _, test := range tests {
		out := test.fr.Transform(test.conf)["library_frame"]
		if test.libraryFrame == nil {
			assert.Nil(t, out, test.msg)
		} else {
			assert.Equal(t, *test.libraryFrame, out, test.msg)
		}
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
