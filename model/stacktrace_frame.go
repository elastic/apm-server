package model

import (
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
	Original  Original
}

type Sourcemap struct {
	Updated *bool
	Error   *string
}

type Original struct {
	AbsPath      *string
	Filename     string
	Lineno       int
	Colno        *int
	Function     *string
	LibraryFrame *bool

	sourcemapCopied bool
}

func (s *StacktraceFrame) Transform(config *pr.Config) common.MapStr {
	enhancer := utility.NewMapStrEnhancer()
	m := common.MapStr{}
	enhancer.Add(m, "filename", s.Filename)
	enhancer.Add(m, "abs_path", s.AbsPath)
	enhancer.Add(m, "module", s.Module)
	enhancer.Add(m, "function", s.Function)
	enhancer.Add(m, "vars", s.Vars)
	if config != nil && config.LibraryPattern != nil {
		s.setLibraryFrame(config.LibraryPattern)
	}
	enhancer.Add(m, "library_frame", s.LibraryFrame)

	if config != nil && config.ExcludeFromGrouping != nil {
		s.setExcludeFromGrouping(config.ExcludeFromGrouping)
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

	orig := common.MapStr{}
	enhancer.Add(orig, "library_frame", s.Original.LibraryFrame)
	if s.Sourcemap.Updated != nil && *(s.Sourcemap.Updated) {
		enhancer.Add(orig, "filename", s.Original.Filename)
		enhancer.Add(orig, "abs_path", s.Original.AbsPath)
		enhancer.Add(orig, "function", s.Original.Function)
		enhancer.Add(orig, "colno", s.Original.Colno)
		enhancer.Add(orig, "lineno", s.Original.Lineno)
	}
	enhancer.Add(m, "original", orig)

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

func (s *StacktraceFrame) applySourcemap(mapper sourcemap.Mapper, service Service, prevFunction string) string {
	s.setOriginalSourcemapData()

	if s.Original.Colno == nil {
		s.updateError("Colno mandatory for sourcemapping.")
		return prevFunction
	}
	sourcemapId := s.buildSourcemapId(service)
	mapping, err := mapper.Apply(sourcemapId, s.Original.Lineno, *s.Original.Colno)
	if err != nil {
		logp.NewLogger("stacktrace").Errorf("failed to apply sourcemap %s", err.Error())
		e, isSourcemapError := err.(sourcemap.Error)
		if !isSourcemapError || e.Kind == sourcemap.MapError || e.Kind == sourcemap.KeyError {
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
	logp.NewLogger("stacktrace").Error(errMsg)
	s.Sourcemap.Error = &errMsg
	s.updateSmap(false)
}

func (s *StacktraceFrame) updateSmap(updated bool) {
	s.Sourcemap.Updated = &updated
}
