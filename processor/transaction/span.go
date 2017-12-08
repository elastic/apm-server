package transaction

import (
	"time"

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
	Stacktrace m.Stacktrace `mapstructure:"stacktrace"`
	Context    common.MapStr
	Parent     *int
}

func (s *Span) DocType() string {
	return "span"
}

func (s *Span) Transform() common.MapStr {
	enhancer := utility.NewMapStrEnhancer()
	tr := common.MapStr{}
	enhancer.Add(tr, "id", s.Id)
	enhancer.Add(tr, "name", s.Name)
	enhancer.Add(tr, "type", s.Type)
	enhancer.Add(tr, "start", utility.MillisAsMicros(s.Start))
	enhancer.Add(tr, "duration", utility.MillisAsMicros(s.Duration))
	enhancer.Add(tr, "parent", s.Parent)
	st, onlyLib := s.Stacktrace.Transform()
	if len(st) > 0 {
		enhancer.Add(tr, "stacktrace", st)
		enhancer.Add(tr, "only_library_frames", onlyLib)
	}
	return tr
}

func (s *Span) Mappings(pa *payload, tx Event) (time.Time, []m.DocMapping) {
	return tx.Timestamp,
		[]m.DocMapping{
			{Key: "processor", Apply: func() common.MapStr {
				return common.MapStr{"name": processorName, "event": s.DocType()}
			}},
			{Key: s.DocType(), Apply: func() common.MapStr { return s.Transform() }},
			{Key: "transaction", Apply: func() common.MapStr { return common.MapStr{"id": tx.Id} }},
			{Key: "context", Apply: func() common.MapStr { return s.Context }},
			{Key: "context.service", Apply: pa.Service.MinimalTransform},
		}
}
