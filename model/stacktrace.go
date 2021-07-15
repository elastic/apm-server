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
	"github.com/elastic/beats/v7/libbeat/common"
)

type Stacktrace []*StacktraceFrame

type StacktraceFrame struct {
	AbsPath      string
	Filename     string
	Classname    string
	Lineno       *int
	Colno        *int
	ContextLine  string
	Module       string
	Function     string
	LibraryFrame bool
	Vars         common.MapStr
	PreContext   []string
	PostContext  []string

	ExcludeFromGrouping bool

	SourcemapUpdated bool
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
	LibraryFrame bool
}

func (st Stacktrace) transform() []common.MapStr {
	if len(st) == 0 {
		return nil
	}
	frames := make([]common.MapStr, len(st))
	for i, frame := range st {
		frames[i] = frame.transform()
	}
	return frames
}

func (s *StacktraceFrame) transform() common.MapStr {
	var m mapStr
	m.maybeSetString("filename", s.Filename)
	m.maybeSetString("classname", s.Classname)
	m.maybeSetString("abs_path", s.AbsPath)
	m.maybeSetString("module", s.Module)
	m.maybeSetString("function", s.Function)
	m.maybeSetMapStr("vars", s.Vars)

	if s.LibraryFrame {
		m.set("library_frame", s.LibraryFrame)
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
	if s.SourcemapUpdated {
		sm.set("updated", true)
	}
	sm.maybeSetString("error", s.SourcemapError)
	m.maybeSetMapStr("sourcemap", common.MapStr(sm))

	var orig mapStr
	if s.Original.LibraryFrame {
		orig.set("library_frame", s.Original.LibraryFrame)
	}
	if s.SourcemapUpdated {
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
