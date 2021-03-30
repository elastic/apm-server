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
	"context"
	"fmt"
	"regexp"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

const (
	errMsgSourcemapColumnMandatory = "Colno mandatory for sourcemapping."
	errMsgSourcemapLineMandatory   = "Lineno mandatory for sourcemapping."
	errMsgSourcemapPathMandatory   = "AbsPath mandatory for sourcemapping."
)

type StacktraceFrame struct {
	AbsPath      string
	Filename     string
	Classname    string
	Lineno       *int
	Colno        *int
	ContextLine  string
	Module       string
	Function     string
	LibraryFrame *bool
	Vars         common.MapStr
	PreContext   []string
	PostContext  []string

	ExcludeFromGrouping bool

	SourcemapUpdated *bool
	SourcemapError   string
	Original         Original
}

type Original struct {
	AbsPath      string
	Filename     string
	Classname    string
	Lineno       *int
	Colno        *int
	Function     string
	LibraryFrame *bool

	sourcemapCopied bool
}

func (s *StacktraceFrame) transform(cfg *transform.Config, rum bool) common.MapStr {
	var m mapStr
	m.maybeSetString("filename", s.Filename)
	m.maybeSetString("classname", s.Classname)
	m.maybeSetString("abs_path", s.AbsPath)
	m.maybeSetString("module", s.Module)
	m.maybeSetString("function", s.Function)
	m.maybeSetMapStr("vars", s.Vars)

	if rum && cfg.RUM.LibraryPattern != nil {
		s.setLibraryFrame(cfg.RUM.LibraryPattern)
	}
	if s.LibraryFrame != nil {
		m.set("library_frame", *s.LibraryFrame)
	}

	if rum && cfg.RUM.ExcludeFromGrouping != nil {
		s.setExcludeFromGrouping(cfg.RUM.ExcludeFromGrouping)
	}
	m.set("exclude_from_grouping", s.ExcludeFromGrouping)

	var context mapStr
	if len(s.PreContext) > 0 {
		context.set("pre", s.PreContext)
	}
	if len(s.PostContext) > 0 {
		context.set("post", s.PostContext)
	}
	m.maybeSetMapStr("context", common.MapStr(context))

	var line mapStr
	line.maybeSetIntptr("number", s.Lineno)
	line.maybeSetIntptr("column", s.Colno)
	line.maybeSetString("context", s.ContextLine)
	m.maybeSetMapStr("line", common.MapStr(line))

	var sm mapStr
	sm.maybeSetBool("updated", s.SourcemapUpdated)
	sm.maybeSetString("error", s.SourcemapError)
	m.maybeSetMapStr("sourcemap", common.MapStr(sm))

	var orig mapStr
	orig.maybeSetBool("library_frame", s.Original.LibraryFrame)
	if s.SourcemapUpdated != nil && *(s.SourcemapUpdated) {
		orig.maybeSetString("filename", s.Original.Filename)
		orig.maybeSetString("classname", s.Original.Classname)
		orig.maybeSetString("abs_path", s.Original.AbsPath)
		orig.maybeSetString("function", s.Original.Function)
		orig.maybeSetIntptr("colno", s.Original.Colno)
		orig.maybeSetIntptr("lineno", s.Original.Lineno)
	}
	m.maybeSetMapStr("original", common.MapStr(orig))

	return common.MapStr(m)
}

func (s *StacktraceFrame) IsLibraryFrame() bool {
	return s.LibraryFrame != nil && *s.LibraryFrame
}

func (s *StacktraceFrame) IsSourcemapApplied() bool {
	return s.SourcemapUpdated != nil && *s.SourcemapUpdated
}

func (s *StacktraceFrame) setExcludeFromGrouping(pattern *regexp.Regexp) {
	s.ExcludeFromGrouping = s.Filename != "" && pattern.MatchString(s.Filename)
}

func (s *StacktraceFrame) setLibraryFrame(pattern *regexp.Regexp) {
	s.Original.LibraryFrame = s.LibraryFrame
	libraryFrame := (s.Filename != "" && pattern.MatchString(s.Filename)) ||
		(s.AbsPath != "" && pattern.MatchString(s.AbsPath))
	s.LibraryFrame = &libraryFrame
}

func (s *StacktraceFrame) applySourcemap(
	ctx context.Context,
	store *sourcemap.Store,
	service *Service,
	prevFunction string,
	maxLineLength int,
) (function, errMsg string) {
	function = prevFunction

	var valid bool
	if valid, errMsg = s.validForSourcemapping(); !valid {
		s.updateError(errMsg)
		return
	}

	s.setOriginalSourcemapData()

	path := utility.CleanUrlPath(s.Original.AbsPath)
	mapper, err := store.Fetch(ctx, service.Name, service.Version, path)
	if err != nil {
		errMsg = err.Error()
		return
	}
	if mapper == nil {
		errMsg = fmt.Sprintf("No Sourcemap available for ServiceName %s, ServiceVersion %s, Path %s.",
			service.Name, service.Version, path)
		s.updateError(errMsg)
		return
	}

	file, fct, line, col, ctxLine, preCtx, postCtx, ok := sourcemap.Map(mapper, *s.Original.Lineno, *s.Original.Colno, maxLineLength)
	if !ok {
		errMsg = fmt.Sprintf("No Sourcemap found for Lineno %v, Colno %v", *s.Original.Lineno, *s.Original.Colno)
		s.updateError(errMsg)
		return
	}

	if file != "" {
		s.Filename = file
	}
	s.Colno = &col
	s.Lineno = &line
	s.AbsPath = path
	s.updateSmap(true)
	s.Function = prevFunction
	s.ContextLine = ctxLine
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
	if s.AbsPath == "" {
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
	s.SourcemapError = errMsg
	s.updateSmap(false)
}

func (s *StacktraceFrame) updateSmap(updated bool) {
	s.SourcemapUpdated = &updated
}
