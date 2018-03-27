package transaction

import (
	"errors"

	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Span struct {
	Id         *int
	Name       string
	Type       string
	Start      float64
	Duration   float64
	Context    common.MapStr
	Parent     *int
	Stacktrace m.Stacktrace
}

func DecodeSpan(input interface{}, err error) (*Span, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for span")
	}
	decoder := utility.ManualDecoder{}
	sp := Span{
		Id:       decoder.IntPtr(raw, "id"),
		Name:     decoder.String(raw, "name"),
		Type:     decoder.String(raw, "type"),
		Start:    decoder.Float64(raw, "start"),
		Duration: decoder.Float64(raw, "duration"),
		Context:  decoder.MapStr(raw, "context"),
		Parent:   decoder.IntPtr(raw, "parent"),
	}
	var stacktr *m.Stacktrace
	stacktr, err = m.DecodeStacktrace(raw["stacktrace"], decoder.Err)
	if stacktr != nil {
		sp.Stacktrace = *stacktr
	}
	return &sp, err
}

func (s *Span) Transform(config config.Config, service m.Service) common.MapStr {
	if s == nil {
		return nil
	}
	tr := common.MapStr{}
	utility.Add(tr, "id", s.Id)
	utility.Add(tr, "name", s.Name)
	utility.Add(tr, "type", s.Type)
	utility.Add(tr, "start", utility.MillisAsMicros(s.Start))
	utility.Add(tr, "duration", utility.MillisAsMicros(s.Duration))
	utility.Add(tr, "parent", s.Parent)
	st := s.Stacktrace.Transform(config, service)
	utility.Add(tr, "stacktrace", st)
	return tr
}
