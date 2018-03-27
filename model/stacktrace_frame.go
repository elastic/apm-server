package model

import (
	"errors"
	"regexp"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

type StacktraceFrame struct {
	AbsPath      *string
	Filename     string
	Lineno       int
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
	Lineno       int
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
		return nil, errors.New("Invalid type for stacktrace frame")
	}
	decoder := utility.ManualDecoder{}
	frame := StacktraceFrame{
		AbsPath:      decoder.StringPtr(raw, "abs_path"),
		Filename:     decoder.String(raw, "filename"),
		Lineno:       decoder.Int(raw, "lineno"),
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

func (s *StacktraceFrame) Transform(config config.Config) common.MapStr {
	m := common.MapStr{}
	utility.Add(m, "filename", s.Filename)
	utility.Add(m, "abs_path", s.AbsPath)
	utility.Add(m, "module", s.Module)
	utility.Add(m, "function", s.Function)
	utility.Add(m, "vars", s.Vars)
	if config.LibraryPattern != nil {
		s.setLibraryFrame(config.LibraryPattern)
	}
	utility.Add(m, "library_frame", s.LibraryFrame)

	if config.ExcludeFromGrouping != nil {
		s.setExcludeFromGrouping(config.ExcludeFromGrouping)
	}
	utility.Add(m, "exclude_from_grouping", s.ExcludeFromGrouping)

	context := common.MapStr{}
	utility.Add(context, "pre", s.PreContext)
	utility.Add(context, "post", s.PostContext)
	utility.Add(m, "context", context)

	line := common.MapStr{}
	utility.Add(line, "number", s.Lineno)
	utility.Add(line, "column", s.Colno)
	utility.Add(line, "context", s.ContextLine)
	utility.Add(m, "line", line)

	sm := common.MapStr{}
	utility.Add(sm, "updated", s.Sourcemap.Updated)
	utility.Add(sm, "error", s.Sourcemap.Error)
	utility.Add(m, "sourcemap", sm)

	orig := common.MapStr{}
	utility.Add(orig, "library_frame", s.Original.LibraryFrame)
	if s.Sourcemap.Updated != nil && *(s.Sourcemap.Updated) {
		utility.Add(orig, "filename", s.Original.Filename)
		utility.Add(orig, "abs_path", s.Original.AbsPath)
		utility.Add(orig, "function", s.Original.Function)
		utility.Add(orig, "colno", s.Original.Colno)
		utility.Add(orig, "lineno", s.Original.Lineno)
	}
	utility.Add(m, "original", orig)

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
