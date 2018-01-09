package model

import (
	"fmt"
	"regexp"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/sourcemap"
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

func (s *StacktraceFrame) Transform(config *pr.Config, service Service) common.MapStr {
	enhancer := utility.NewMapStrEnhancer()
	m := common.MapStr{}

	if config != nil && config.SmapMapper != nil {
		sm := common.MapStr{}
		s.applySourcemap(service, config.SmapMapper)
		enhancer.Add(sm, "updated", s.SourcemapUpdated)
		enhancer.Add(sm, "error", s.SourcemapError)
		enhancer.Add(m, "sourcemap", sm)
	}
	enhancer.Add(m, "filename", s.Filename)
	enhancer.Add(m, "abs_path", s.AbsPath)
	enhancer.Add(m, "module", s.Module)
	enhancer.Add(m, "function", s.Function)
	enhancer.Add(m, "vars", s.Vars)
	if config != nil && config.LibraryPattern != nil && s.LibraryFrame == nil {
		s.LibraryFrame = s.isLibraryFrame(config.LibraryPattern)
	}
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

func (s *StacktraceFrame) isLibraryFrame(pattern *regexp.Regexp) *bool {
	matching := pattern.MatchString(s.Filename) ||
		(s.AbsPath != nil && pattern.MatchString(*s.AbsPath))
	return &matching
}

func (s *StacktraceFrame) applySourcemap(service Service, mapper sourcemap.Mapper) {
	if s.Colno == nil {
		s.updateError("Colno mandatory for sourcemapping.")
		return
	}
	sourcemapId := s.buildSourcemapId(service)
	mapping, err := mapper.Apply(sourcemapId, s.Lineno, *s.Colno)
	if err != nil {
		logp.Err(fmt.Sprintf("Sourcemap fetching Error %s", err.Error()))
		e, issourcemapError := err.(sourcemap.Error)
		if !issourcemapError || e.Kind == sourcemap.MapError || e.Kind == sourcemap.KeyError {
			s.updateError(err.Error())
		}
		return
	}

	if mapping.Filename != "" {
		s.Filename = mapping.Filename
	}
	if mapping.Function != "" {
		s.Function = &mapping.Function
	}
	s.Colno = &mapping.Colno
	s.Lineno = mapping.Lineno
	s.AbsPath = &mapping.Path
	s.SourcemapUpdated = true
}

func (s *StacktraceFrame) buildSourcemapId(service Service) sourcemap.Id {
	id := sourcemap.Id{ServiceName: service.Name}
	if service.Version != nil {
		id.ServiceVersion = *service.Version
	}
	if s.AbsPath != nil {
		id.Path = utility.CleanUrlPath(*s.AbsPath)
	}
	return id
}

func (s *StacktraceFrame) updateError(errMsg string) {
	logp.Err(errMsg)
	s.SourcemapError = &errMsg
	s.SourcemapUpdated = false
}
