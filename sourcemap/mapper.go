package sourcemap

import (
	"fmt"
	"time"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

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
			Msg: fmt.Sprintf("No Sourcemap found for Id %v, Lineno %v, Colno %v",
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
	_, err := m.Accessor.Fetch(id)
	if err == nil {
		logp.NewLogger("sourcemap").Warnf("Overriding sourcemap for service %s version %s and file %s",
			id.ServiceName, id.ServiceVersion, id.Path)
	}
	m.Accessor.Remove(id)
}
