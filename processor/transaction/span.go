package transaction

import (
	"errors"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
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
	df := utility.DataFetcher{}
	sp := Span{
		Id:       df.IntPtr(raw, "id"),
		Name:     df.String(raw, "name"),
		Type:     df.String(raw, "type"),
		Start:    df.Float64(raw, "start"),
		Duration: df.Float64(raw, "duration"),
		Context:  df.MapStr(raw, "context"),
		Parent:   df.IntPtr(raw, "parent"),
	}
	var stacktr *m.Stacktrace
	stacktr, err = m.DecodeStacktrace(raw["stacktrace"], df.Err)
	if stacktr != nil {
		sp.Stacktrace = *stacktr
	}
	return &sp, err
}

func (s *Span) Transform(config *pr.Config, service m.Service) common.MapStr {
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
