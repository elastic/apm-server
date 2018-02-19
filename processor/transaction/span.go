package transaction

import (
	"time"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Span struct {
	Name     *string
	Type     *string
	Start    *float64
	Duration *float64

	Id         *int
	Stacktrace m.Stacktrace `mapstructure:"stacktrace"`
	Context    common.MapStr
	Parent     *int
}

const spanDocType = "span"

func (s *Span) Mappings(config *pr.Config, pa *payload, tx *Event) (time.Time, []utility.DocMapping) {
	return tx.Timestamp,
		[]utility.DocMapping{
			{Key: "processor", Apply: func() common.MapStr {
				return common.MapStr{"name": processorName, "event": spanDocType}
			}},
			{Key: spanDocType, Apply: func() common.MapStr { return s.Transform(config, pa.Service) }},
			{Key: "transaction", Apply: func() common.MapStr { return common.MapStr{"id": tx.Id} }},
			{Key: "context", Apply: func() common.MapStr { return s.Context }},
			{Key: "context.service", Apply: pa.Service.MinimalTransform},
		}
}

func (s *Span) Transform(config *pr.Config, service m.Service) common.MapStr {
	if s == nil {
		return nil
	}
	tr := common.MapStr{}
	utility.AddIntPtr(tr, "id", s.Id)
	utility.AddStrPtr(tr, "name", s.Name)
	utility.AddStrPtr(tr, "type", s.Type)
	utility.AddMillis(tr, "start", s.Start)
	utility.AddMillis(tr, "duration", s.Duration)
	utility.AddIntPtr(tr, "parent", s.Parent)
	st := s.Stacktrace.Transform(config, service)
	if len(st) > 0 {
		utility.AddCommonMapStrArray(tr, "stacktrace", st)
	}
	if len(tr) == 0 {
		return nil
	}
	return tr
}
