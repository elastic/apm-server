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
	"errors"
	"regexp"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type StacktraceFrame struct {
	AbsPath      *string
	Filename     string
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
	Filename     string
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
		return nil, errors.New("invalid type for stacktrace frame")
	}
	decoder := utility.ManualDecoder{}
	frame := StacktraceFrame{
		AbsPath:      decoder.StringPtr(raw, "abs_path"),
		Filename:     decoder.String(raw, "filename"),
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

func (s *StacktraceFrame) Transform(tctx *transform.Context) common.MapStr {
	m := common.MapStr{}
	utility.Set(m, "filename", s.Filename)
	utility.Set(m, "abs_path", s.AbsPath)
	utility.Set(m, "module", s.Module)
	utility.Set(m, "function", s.Function)
	utility.Set(m, "vars", s.Vars)
	if tctx.Config.LibraryPattern != nil {
		s.setLibraryFrame(tctx.Config.LibraryPattern)
	}
	utility.Set(m, "library_frame", s.LibraryFrame)

	if tctx.Config.ExcludeFromGrouping != nil {
		s.setExcludeFromGrouping(tctx.Config.ExcludeFromGrouping)
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
	s.ExcludeFromGrouping = pattern.MatchString(s.Filename)
}

func (s *StacktraceFrame) setLibraryFrame(pattern *regexp.Regexp) {
	s.Original.LibraryFrame = s.LibraryFrame
	libraryFrame := pattern.MatchString(s.Filename) ||
		(s.AbsPath != nil && pattern.MatchString(*s.AbsPath))
	s.LibraryFrame = &libraryFrame
}

func (s *StacktraceFrame) applySourcemap(mapper sourcemap.Mapper, service *metadata.Service, prevFunction string) (string, string) {
	s.setOriginalSourcemapData()

	if s.Original.Colno == nil {
		errMsg := "Colno mandatory for sourcemapping."
		s.updateError(errMsg)
		return prevFunction, errMsg
	}
	if s.Original.Lineno == nil {
		errMsg := "Lineno mandatory for sourcemapping."
		s.updateError(errMsg)
		return prevFunction, errMsg
	}

	sourcemapId, errMsg := s.buildSourcemapId(service)
	if errMsg != "" {
		return prevFunction, errMsg
	}
	mapping, err := mapper.Apply(sourcemapId, *s.Original.Lineno, *s.Original.Colno)
	if err != nil {
		e, isSourcemapError := err.(sourcemap.Error)
		if !isSourcemapError || e.Kind == sourcemap.MapError || e.Kind == sourcemap.KeyError {
			s.updateError(err.Error())
		}
		return prevFunction, err.Error()
	}

	if mapping.Filename != "" {
		s.Filename = mapping.Filename
	}

	s.Colno = &mapping.Colno
	s.Lineno = &mapping.Lineno
	s.AbsPath = &mapping.Path
	s.updateSmap(true)
	s.Function = &prevFunction
	s.ContextLine = &mapping.ContextLine
	s.PreContext = mapping.PreContext
	s.PostContext = mapping.PostContext

	if mapping.Function != "" {
		return mapping.Function, ""
	}
	return "<unknown>", ""
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

	s.Original.sourcemapCopied = true
}

func (s *StacktraceFrame) buildSourcemapId(service *metadata.Service) (sourcemap.Id, string) {
	if service.Name == nil {
		return sourcemap.Id{}, "Cannot apply sourcemap without a service name."
	}
	id := sourcemap.Id{ServiceName: *service.Name}
	if service.Version != nil {
		id.ServiceVersion = *service.Version
	}
	if s.AbsPath != nil {
		id.Path = utility.CleanUrlPath(*s.AbsPath)
	}
	return id, ""
}

func (s *StacktraceFrame) updateError(errMsg string) {
	s.Sourcemap.Error = &errMsg
	s.updateSmap(false)
}

func (s *StacktraceFrame) updateSmap(updated bool) {
	s.Sourcemap.Updated = &updated
}
