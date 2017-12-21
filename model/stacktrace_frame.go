package model

import (
	"fmt"

	"github.com/go-sourcemap/sourcemap"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

type StacktraceFrame struct {
	AbsPath      *string `mapstructure:"abs_path"`
	Filename     string
	Lineno       int
	Colno        *int
	ContextLine  *string `mapstructure:"context_line"`
	Module       *string
	Function     *string
	LibraryFrame *bool `mapstructure:"library_frame"`
	Vars         common.MapStr
	PreContext   []string `mapstructure:"pre_context"`
	PostContext  []string `mapstructure:"post_context"`

	SourcemapUpdated bool
	SourcemapError   *string
}

func (s *StacktraceFrame) Transform(service Service, smapAccessor utility.SmapAccessor) common.MapStr {
	enhancer := utility.NewMapStrEnhancer()
	m := common.MapStr{}

	if smapAccessor != nil {
		sm := common.MapStr{}
		s.applySourcemap(service, smapAccessor)
		enhancer.Add(sm, "updated", s.SourcemapUpdated)
		enhancer.Add(sm, "error", s.SourcemapError)
		enhancer.Add(m, "sourcemap", sm)
	}
	enhancer.Add(m, "filename", s.Filename)
	enhancer.Add(m, "abs_path", s.AbsPath)
	enhancer.Add(m, "module", s.Module)
	enhancer.Add(m, "function", s.Function)
	enhancer.Add(m, "vars", s.Vars)
	enhancer.Add(m, "library_frame", s.LibraryFrame)

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

func (s *StacktraceFrame) applySourcemap(service Service, smapAccessor utility.SmapAccessor) {
	if service.Name == "" ||
		service.Version == nil ||
		s.AbsPath == nil ||
		s.Colno == nil {
		s.updateError("AbsPath, Colno, Service Name and Version mandatory for sourcemapping.")
		return
	}

	cleanedPath := utility.CleanUrlPath(*s.AbsPath)
	smapConsumer, err := smapAccessor.Fetch(utility.SmapID{
		ServiceName:    service.Name,
		ServiceVersion: *service.Version,
		Path:           cleanedPath,
	})
	if err != nil {
		e, isSmapError := err.(utility.SmapError)
		if !isSmapError || e.Kind == utility.MapError {
			s.updateError(err.Error())
		}
		return
	}
	if smapConsumer == nil {
		s.updateError("No Sourcemap found for this StacktraceFrame.")
		return
	}
	s.updateMappings(*smapConsumer, cleanedPath)
}

func (s *StacktraceFrame) updateMappings(smapCons sourcemap.Consumer, cleanedPath string) {
	fileName, funcName, line, col, ok := smapCons.Source(s.Lineno, *s.Colno)
	if ok == true {
		ln := line + 1 // account for 0 based line numbering
		s.Filename = fileName
		s.Function = &funcName
		s.Lineno = ln
		s.Colno = &col
		s.AbsPath = &cleanedPath
		s.SourcemapUpdated = true
	} else {
		s.updateError(fmt.Sprintf("No mapping for Lineno %v and Colno %v.", s.Lineno, *s.Colno))
	}
}

func (s *StacktraceFrame) updateError(errMsg string) {
	logp.Err(errMsg)
	s.SourcemapError = &errMsg
	s.SourcemapUpdated = false
}
