package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type StacktraceFrame struct {
	AbsPath     *string       `json:"abs_path"`
	Filename    string        `json:"filename"`
	Lineno      int           `json:"lineno"`
	Colno       *int          `json:"colno"`
	ContextLine *string       `json:"context_line"`
	Module      *string       `json:"module"`
	Function    *string       `json:"function"`
	InApp       *bool         `json:"in_app"`
	Vars        common.MapStr `json:"vars"`
	PreContext  []string      `json:"pre_context"`
	PostContext []string      `json:"post_context"`
}

func TransformFrame(s *StacktraceFrame, _ App) common.MapStr {
	enhancer := utility.NewMapStrEnhancer()
	m := common.MapStr{}

	enhancer.Add(m, "filename", s.Filename)
	enhancer.Add(m, "abs_path", s.AbsPath)
	enhancer.Add(m, "module", s.Module)
	enhancer.Add(m, "function", s.Function)
	enhancer.Add(m, "vars", s.Vars)
	enhancer.Add(m, "in_app", s.InApp)

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
