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

	ExcludeFromGrouping bool

	Sourcemap Sourcemap
}

type Sourcemap struct {
	Updated *bool
	Error   *string
}

func (s *StacktraceFrame) Transform(config *pr.Config) common.MapStr {
	enhancer := utility.NewMapStrEnhancer()
	m := common.MapStr{}
	enhancer.Add(m, "filename", s.Filename)
	enhancer.Add(m, "abs_path", s.AbsPath)
	enhancer.Add(m, "module", s.Module)
	enhancer.Add(m, "function", s.Function)
	enhancer.Add(m, "vars", s.Vars)
	if config != nil && config.LibraryPattern != nil && s.LibraryFrame == nil {
		libraryFrame := s.isLibraryFrame(config.LibraryPattern)
		s.LibraryFrame = &libraryFrame
	}
	enhancer.Add(m, "library_frame", s.LibraryFrame)

	if config != nil && config.ExcludeFromGrouping != nil {
		s.ExcludeFromGrouping = s.isExcludedFromGrouping(config.ExcludeFromGrouping)
	}
	enhancer.Add(m, "exclude_from_grouping", s.ExcludeFromGrouping)

	context := common.MapStr{}
	enhancer.Add(context, "pre", s.PreContext)
	enhancer.Add(context, "post", s.PostContext)
	enhancer.Add(m, "context", context)

	line := common.MapStr{}
	enhancer.Add(line, "number", s.Lineno)
	enhancer.Add(line, "column", s.Colno)
	enhancer.Add(line, "context", s.ContextLine)
	enhancer.Add(m, "line", line)

	sm := common.MapStr{}
	enhancer.Add(sm, "updated", s.Sourcemap.Updated)
	enhancer.Add(sm, "error", s.Sourcemap.Error)
	enhancer.Add(m, "sourcemap", sm)

	return m
}

func (s *StacktraceFrame) isExcludedFromGrouping(pattern *regexp.Regexp) bool {
	return pattern.MatchString(s.Filename)
}

func (s *StacktraceFrame) isLibraryFrame(pattern *regexp.Regexp) bool {
	return pattern.MatchString(s.Filename) ||
		(s.AbsPath != nil && pattern.MatchString(*s.AbsPath))
}

func (s *StacktraceFrame) applySourcemap(mapper sourcemap.Mapper, service Service, prevFunction string) string {
	if s.Colno == nil {
		s.updateError("Colno mandatory for sourcemapping.")
		return prevFunction
	}
	sourcemapId := s.buildSourcemapId(service)
	mapping, err := mapper.Apply(sourcemapId, s.Lineno, *s.Colno)
	if err != nil {
		logp.Err(fmt.Sprintf("failed to apply sourcemap %s", err.Error()))
		e, issourcemapError := err.(sourcemap.Error)
		if !issourcemapError || e.Kind == sourcemap.MapError || e.Kind == sourcemap.KeyError {
			s.updateError(err.Error())
		}
		return prevFunction
	}

	if mapping.Filename != "" {
		s.Filename = mapping.Filename
	}

	s.Colno = &mapping.Colno
	s.Lineno = mapping.Lineno
	s.AbsPath = &mapping.Path
	s.updateSmap(true)
	s.Function = &prevFunction

	if mapping.Function != "" {
		return mapping.Function
	}
	return "<unknown>"
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
	s.Sourcemap.Error = &errMsg
	s.updateSmap(false)
}

func (s *StacktraceFrame) updateSmap(updated bool) {
	s.Sourcemap.Updated = &updated
}
