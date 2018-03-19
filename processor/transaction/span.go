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

func (sp *Span) decode(input interface{}) error {
	if input == nil || sp == nil {
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return errors.New("Invalid type for span")
	}
	df := utility.DataFetcher{}
	sp.Id = df.IntPtr(raw, "id")
	sp.Name = df.String(raw, "name")
	sp.Type = df.String(raw, "type")
	sp.Start = df.Float64(raw, "start")
	sp.Duration = df.Float64(raw, "duration")
	sp.Context = df.MapStr(raw, "context")
	sp.Parent = df.IntPtr(raw, "parent")
	sp.Stacktrace = m.Stacktrace{}
	if err := sp.Stacktrace.Decode(raw["stacktrace"]); err != nil {
		return err
	}
	return df.Err
}

func (s *Span) Transform(config *pr.Config, service m.Service) common.MapStr {
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
