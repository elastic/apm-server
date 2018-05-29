package sourcemap

import (
	"fmt"
	"time"

	"strings"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

const sourcemapContentSnippetSize = 5

type Mapper interface {
	Apply(Id, int, int) (*Mapping, error)
	NewSourcemapAdded(id Id)
}

type SmapMapper struct {
	Accessor Accessor
}

type Config struct {
	CacheExpiration     time.Duration
	ElasticsearchConfig *common.Config
	Index               string
}

type Mapping struct {
	Filename    string
	Function    string
	Colno       int
	Lineno      int
	Path        string
	ContextLine string
	PreContext  []string
	PostContext []string
}

func NewSmapMapper(config Config) (*SmapMapper, error) {
	accessor, err := NewSmapAccessor(config)
	if err != nil {
		return nil, err
	}
	return &SmapMapper{Accessor: accessor}, nil
}

func (m *SmapMapper) Apply(id Id, lineno, colno int) (*Mapping, error) {
	smapCons, err := m.Accessor.Fetch(id)
	if err != nil {
		return nil, err
	}

	file, funct, line, col, ok := smapCons.Source(lineno, colno)
	if !ok {
		return nil, Error{
			Msg: fmt.Sprintf("No Sourcemap found for Id %v, Lineno %v, Colno %v",
				id.String(), lineno, colno),
			Kind: KeyError,
		}
	}
	src := strings.Split(smapCons.SourceContent(file), "\n")
	return &Mapping{
		Filename: file,
		Function: funct,
		Lineno:   line,
		Colno:    col,
		Path:     id.Path,
		// line is 1-based
		ContextLine: strings.Join(subSlice(line-1, line, src), ""),
		PreContext:  subSlice(line-1-sourcemapContentSnippetSize, line-1, src),
		PostContext: subSlice(line, line+sourcemapContentSnippetSize, src),
	}, nil
}

func (m *SmapMapper) NewSourcemapAdded(id Id) {
	_, err := m.Accessor.Fetch(id)
	if err == nil {
		logp.NewLogger("sourcemap").Warnf("Overriding sourcemap for service %s version %s and file %s",
			id.ServiceName, id.ServiceVersion, id.Path)
	}
	m.Accessor.Remove(id)
}

func subSlice(from, to int, content []string) []string {
	if len(content) == 0 {
		return content
	}
	if from < 0 {
		from = 0
	}
	if to > len(content) {
		to = len(content)
	}
	return content[from:to]
}
