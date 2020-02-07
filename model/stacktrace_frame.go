// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package model

import (
	"fmt"
	"regexp"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/utility"
)

const (
	errMsgSourcemapColumnMandatory = "Colno mandatory for sourcemapping."
	errMsgSourcemapLineMandatory   = "Lineno mandatory for sourcemapping."
	errMsgSourcemapPathMandatory   = "AbsPath mandatory for sourcemapping."
)

var (
	errInvalidStacktraceFrameType = errors.New("invalid type for stacktrace frame")
)

type StacktraceFrame struct {
	AbsPath      *string
	Filename     *string
	Classname    *string
	Lineno       *int
	Colno        *int
	ContextLine  *string
	Module       *string
	Function     *string
	LibraryFrame *bool
	Vars         common.MapStr
	PreContext   []string
	PostContext  []string

	ExcludeFromGrouping bool

	Sourcemap Sourcemap
	Original  Original
}

type Sourcemap struct {
	Updated *bool
	Error   *string
}

type Original struct {
	AbsPath      *string
	Filename     *string
	Classname    *string
	Lineno       *int
	Colno        *int
	Function     *string
	LibraryFrame *bool

	sourcemapCopied bool
}

func DecodeStacktraceFrame(input interface{}, err error) (*StacktraceFrame, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errInvalidStacktraceFrameType
	}
	decoder := utility.ManualDecoder{}
	frame := StacktraceFrame{
		AbsPath:      decoder.StringPtr(raw, "abs_path"),
		Filename:     decoder.StringPtr(raw, "filename"),
		Classname:    decoder.StringPtr(raw, "classname"),
		Lineno:       decoder.IntPtr(raw, "lineno"),
		Colno:        decoder.IntPtr(raw, "colno"),
		ContextLine:  decoder.StringPtr(raw, "context_line"),
		Module:       decoder.StringPtr(raw, "module"),
		Function:     decoder.StringPtr(raw, "function"),
		LibraryFrame: decoder.BoolPtr(raw, "library_frame"),
		Vars:         decoder.MapStr(raw, "vars"),
		PreContext:   decoder.StringArr(raw, "pre_context"),
		PostContext:  decoder.StringArr(raw, "post_context"),
	}
	return &frame, decoder.Err
}

func (s *StacktraceFrame) transform(libraryPattern, excludeFromGrouping *regexp.Regexp) common.MapStr {
	m := common.MapStr{}
	utility.Set(m, "filename", s.Filename)
	utility.Set(m, "classname", s.Classname)
	utility.Set(m, "abs_path", s.AbsPath)
	utility.Set(m, "module", s.Module)
	utility.Set(m, "function", s.Function)
	utility.Set(m, "vars", s.Vars)
	if libraryPattern != nil {
		s.setLibraryFrame(libraryPattern)
	}
	utility.Set(m, "library_frame", s.LibraryFrame)

	if excludeFromGrouping != nil {
		s.setExcludeFromGrouping(excludeFromGrouping)
	}
	utility.Set(m, "exclude_from_grouping", s.ExcludeFromGrouping)

	context := common.MapStr{}
	utility.Set(context, "pre", s.PreContext)
	utility.Set(context, "post", s.PostContext)
	utility.Set(m, "context", context)

	line := common.MapStr{}
	utility.Set(line, "number", s.Lineno)
	utility.Set(line, "column", s.Colno)
	utility.Set(line, "context", s.ContextLine)
	utility.Set(m, "line", line)

	sm := common.MapStr{}
	utility.Set(sm, "updated", s.Sourcemap.Updated)
	utility.Set(sm, "error", s.Sourcemap.Error)
	utility.Set(m, "sourcemap", sm)

	orig := common.MapStr{}
	utility.Set(orig, "library_frame", s.Original.LibraryFrame)
	if s.Sourcemap.Updated != nil && *(s.Sourcemap.Updated) {
		utility.Set(orig, "filename", s.Original.Filename)
		utility.Set(orig, "classname", s.Original.Classname)
		utility.Set(orig, "abs_path", s.Original.AbsPath)
		utility.Set(orig, "function", s.Original.Function)
		utility.Set(orig, "colno", s.Original.Colno)
		utility.Set(orig, "lineno", s.Original.Lineno)
	}
	utility.Set(m, "original", orig)

	return m
}

func (s *StacktraceFrame) IsLibraryFrame() bool {
	return s.LibraryFrame != nil && *s.LibraryFrame
}

func (s *StacktraceFrame) IsSourcemapApplied() bool {
	return s.Sourcemap.Updated != nil && *s.Sourcemap.Updated
}

func (s *StacktraceFrame) setExcludeFromGrouping(pattern *regexp.Regexp) {
	s.ExcludeFromGrouping = s.Filename != nil && pattern.MatchString(*s.Filename)
}

func (s *StacktraceFrame) setLibraryFrame(pattern *regexp.Regexp) {
	s.Original.LibraryFrame = s.LibraryFrame
	libraryFrame := (s.Filename != nil && pattern.MatchString(*s.Filename)) ||
		(s.AbsPath != nil && pattern.MatchString(*s.AbsPath))
	s.LibraryFrame = &libraryFrame
}

func (s *StacktraceFrame) applySourcemap(store *sourcemap.Store, service *metadata.Service, prevFunction string) (function string, errMsg string) {
	function = prevFunction

	var valid bool
	if valid, errMsg = s.validForSourcemapping(); !valid {
		s.updateError(errMsg)
		return
	}

	s.setOriginalSourcemapData()

	path := utility.CleanUrlPath(*s.Original.AbsPath)
	mapper, err := store.Fetch(*service.Name, *service.Version, path)
	if err != nil {
		errMsg = err.Error()
		return
	}
	if mapper == nil {
		errMsg = fmt.Sprintf("No Sourcemap available for ServiceName %s, ServiceVersion %s, Path %s.",
			*service.Name, *service.Version, path)
		s.updateError(errMsg)
		return
	}

	file, fct, line, col, ctxLine, preCtx, postCtx, ok := sourcemap.Map(mapper, *s.Original.Lineno, *s.Original.Colno)
	if !ok {
		errMsg = fmt.Sprintf("No Sourcemap found for Lineno %v, Colno %v", *s.Original.Lineno, *s.Original.Colno)
		s.updateError(errMsg)
		return
	}

	if file != "" {
		s.Filename = &file
	}

	s.Colno = &col
	s.Lineno = &line
	s.AbsPath = &path
	s.updateSmap(true)
	s.Function = &prevFunction
	s.ContextLine = &ctxLine
	s.PreContext = preCtx
	s.PostContext = postCtx

	if fct != "" {
		function = fct
		return
	}
	function = "<unknown>"
	return
}

func (s *StacktraceFrame) validForSourcemapping() (bool, string) {
	if s.Colno == nil {
		return false, errMsgSourcemapColumnMandatory
	}
	if s.Lineno == nil {
		return false, errMsgSourcemapLineMandatory
	}
	if s.AbsPath == nil {
		return false, errMsgSourcemapPathMandatory
	}
	return true, ""
}

func (s *StacktraceFrame) setOriginalSourcemapData() {
	if s.Original.sourcemapCopied {
		return
	}
	s.Original.Colno = s.Colno
	s.Original.AbsPath = s.AbsPath
	s.Original.Function = s.Function
	s.Original.Lineno = s.Lineno
	s.Original.Filename = s.Filename
	s.Original.Classname = s.Classname

	s.Original.sourcemapCopied = true
}

func (s *StacktraceFrame) updateError(errMsg string) {
	s.Sourcemap.Error = &errMsg
	s.updateSmap(false)
}

func (s *StacktraceFrame) updateSmap(updated bool) {
	s.Sourcemap.Updated = &updated
}
