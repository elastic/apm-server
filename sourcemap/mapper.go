package sourcemap

import (
	"fmt"
	"time"

	"github.com/elastic/beats/libbeat/common"
)

type Mapper interface {
	Apply(Id, int, int) (*Mapping, error)
	NewSourcemapAdded(id Id)
}

type SmapMapper struct {
	Accessor Accessor
}

type Config struct {
	CacheExpiration      time.Duration //seconds
	CacheCleanupInterval time.Duration //seconds
	ElasticsearchConfig  *common.Config
	Index                string
}

type Mapping struct {
	Filename string
	Function string
	Colno    int
	Lineno   int
	Path     string
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
			Msg: fmt.Sprintf("No Sourcemap found for Id %v, lineno %v, colno %v",
				id.String(), lineno, colno),
			Kind: KeyError,
		}
	}

	return &Mapping{
		Filename: file,
		Function: funct,
		Lineno:   line,
		Colno:    col,
		Path:     id.Path,
	}, nil
}

func (m *SmapMapper) NewSourcemapAdded(id Id) {
	m.Accessor.Remove(id)
}
