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
	Filename     *string
	Lineno       *int
	AbsPath      *string `mapstructure:"abs_path"`
	Module       *string
	Function     *string
	LibraryFrame *bool `mapstructure:"library_frame"`
	Colno        *int
	ContextLine  *string `mapstructure:"context_line"`
	Vars         common.MapStr
	PreContext   []string `mapstructure:"pre_context"`
	PostContext  []string `mapstructure:"post_context"`

	ExcludeFromGrouping *bool

	Sourcemap Sourcemap
	Original  Original
}

type Sourcemap struct {
	Updated *bool
	Error   *string
}

type Original struct {
	Filename     *string
	Lineno       *int
	AbsPath      *string
	Colno        *int
	Function     *string
	LibraryFrame *bool

	sourcemapCopied bool
}

func (s *StacktraceFrame) Transform(config *pr.Config) common.MapStr {
	m := common.MapStr{}
	utility.AddStrPtr(m, "filename", s.Filename)
	utility.AddStrPtr(m, "abs_path", s.AbsPath)
	utility.AddStrPtr(m, "module", s.Module)
	utility.AddStrPtr(m, "function", s.Function)
	utility.AddCommonMapStr(m, "vars", s.Vars)

	ma := common.MapStr{}
	utility.AddIntPtr(ma, "number", s.Lineno)
	utility.AddIntPtr(ma, "column", s.Colno)
	utility.AddStrPtr(ma, "context", s.ContextLine)
	utility.AddCommonMapStr(m, "line", ma)

	if config != nil && config.LibraryPattern != nil {
		s.setLibraryFrame(config.LibraryPattern)
	}
	utility.AddBoolPtr(m, "library_frame", s.LibraryFrame)

	s.ExcludeFromGrouping = new(bool)
	if config != nil && config.ExcludeFromGrouping != nil {
		s.setExcludeFromGrouping(config.ExcludeFromGrouping)
	}
	utility.AddBoolPtr(m, "exclude_from_grouping", s.ExcludeFromGrouping)

	ma = common.MapStr{}
	utility.AddStrArray(ma, "pre", s.PreContext)
	utility.AddStrArray(ma, "post", s.PostContext)
	utility.AddCommonMapStr(m, "context", ma)

	ma = common.MapStr{}
	utility.AddBoolPtr(ma, "updated", s.Sourcemap.Updated)
	utility.AddStrPtr(ma, "error", s.Sourcemap.Error)
	utility.AddCommonMapStr(m, "sourcemap", ma)

	ma = common.MapStr{}
	utility.AddBoolPtr(ma, "library_frame", s.Original.LibraryFrame)
	if s.Sourcemap.Updated != nil && *(s.Sourcemap.Updated) {
		utility.AddStrPtr(ma, "filename", s.Original.Filename)
		utility.AddIntPtr(ma, "lineno", s.Original.Lineno)
		utility.AddStrPtr(ma, "abs_path", s.Original.AbsPath)
		utility.AddStrPtr(ma, "function", s.Original.Function)
		utility.AddIntPtr(ma, "colno", s.Original.Colno)
	}
	utility.AddCommonMapStr(m, "original", ma)

	return m
}

func (s *StacktraceFrame) IsLibraryFrame() bool {
	return s.LibraryFrame != nil && *s.LibraryFrame
}

func (s *StacktraceFrame) IsSourcemapApplied() bool {
	return s.Sourcemap.Updated != nil && *s.Sourcemap.Updated
}

func (s *StacktraceFrame) IsExcludedFromGrouping() bool {
	return s.ExcludeFromGrouping != nil && *s.ExcludeFromGrouping
}

func (s *StacktraceFrame) setExcludeFromGrouping(pattern *regexp.Regexp) {
	if s.Filename != nil {
		exclude := pattern.MatchString(*s.Filename)
		s.ExcludeFromGrouping = &exclude
	}
}

func (s *StacktraceFrame) setLibraryFrame(pattern *regexp.Regexp) {
	s.Original.LibraryFrame = s.LibraryFrame
	libraryFrame := (s.Filename != nil && pattern.MatchString(*s.Filename)) ||
		(s.AbsPath != nil && pattern.MatchString(*s.AbsPath))
	s.LibraryFrame = &libraryFrame
}

func (s *StacktraceFrame) applySourcemap(mapper sourcemap.Mapper, service Service, prevFunction string) string {
	s.setOriginalSourcemapData()

	if s.Original.Colno == nil || s.Original.Lineno == nil {
		s.updateError("Colno and Lineno mandatory for sourcemapping.")
		return prevFunction
	}
	sourcemapId := s.buildSourcemapId(service)
	mapping, err := mapper.Apply(sourcemapId, *s.Original.Lineno, *s.Original.Colno)
	if err != nil {
		logp.NewLogger("stacktrace").Errorf("failed to apply sourcemap %s", err.Error())
		e, isSourcemapError := err.(sourcemap.Error)
		if !isSourcemapError || e.Kind == sourcemap.MapError || e.Kind == sourcemap.KeyError {
			s.updateError(err.Error())
		}
		return prevFunction
	}

	if mapping.Filename != "" {
		s.Filename = &mapping.Filename
	}

	s.Colno = &mapping.Colno
	s.Lineno = &mapping.Lineno
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
	id := sourcemap.Id{}
	if service.Name != nil {
		id.ServiceName = *service.Name
	}
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
