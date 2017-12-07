package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type StacktraceFrame struct {
	AbsPath     *string `mapstructure:"abs_path"`
	Filename    string
	Lineno      int
	Colno       *int
	ContextLine *string `mapstructure:"context_line"`
	Module      *string
	Function    *string
	InLibrary   *bool `mapstructure:"library_frame"`
	Vars        common.MapStr
	PreContext  []string `mapstructure:"pre_context"`
	PostContext []string `mapstructure:"post_context"`
}

type TransformStacktraceFrame func(s *StacktraceFrame) common.MapStr

func (s *StacktraceFrame) Transform() common.MapStr {
	enhancer := utility.NewMapStrEnhancer()
	m := common.MapStr{}

	enhancer.Add(m, "filename", s.Filename)
	enhancer.Add(m, "abs_path", s.AbsPath)
	enhancer.Add(m, "module", s.Module)
	enhancer.Add(m, "function", s.Function)
	enhancer.Add(m, "vars", s.Vars)
	enhancer.Add(m, "library_frame", s.InLibrary)

	context := common.MapStr{}
	enhancer.Add(context, "pre", s.PreContext)
	enhancer.Add(context, "post", s.PostContext)
	enhancer.Add(m, "context", context)

	line := common.MapStr{}
	enhancer.Add(line, "number", s.Lineno)
	enhancer.Add(line, "column", s.Colno)
	enhancer.Add(line, "context", s.ContextLine)
	enhancer.Add(m, "line", line)

	return m
}
