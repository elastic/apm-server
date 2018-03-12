package transaction

import (
	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Span struct {
	Id         *int          `json:"id"`
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	Start      float64       `json:"start"`
	Duration   float64       `json:"duration"`
	Stacktrace m.Stacktrace  `json:"stacktrace"`
	Context    common.MapStr `json:"context"`
	Parent     *int          `json:"parent"`
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
