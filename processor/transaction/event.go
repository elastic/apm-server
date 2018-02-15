package transaction

import (
	"time"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

const transactionDocType = "transaction"

type Event struct {
	Id        *string
	Type      *string
	Timestamp time.Time

	Name      *string
	Result    *string
	Duration  *float64
	Context   common.MapStr
	Spans     []*Span
	Marks     common.MapStr
	Sampled   *bool
	SpanCount SpanCount `mapstructure:"span_count"`
}
type SpanCount struct {
	Dropped Dropped
}
type Dropped struct {
	Total *int
}

func (ev *Event) Transform() common.MapStr {
	if ev == nil {
		return nil
	}
	tx := common.MapStr{}
	utility.AddStrPtr(tx, "id", ev.Id)
	utility.AddStrPtr(tx, "name", ev.Name)
	utility.AddMillis(tx, "duration", ev.Duration)
	utility.AddStrPtr(tx, "type", ev.Type)
	utility.AddStrPtr(tx, "result", ev.Result)
	utility.AddCommonMapStr(tx, "marks", ev.Marks)

	if ev.Sampled == nil {
		sampled := true
		ev.Sampled = &sampled
	}
	utility.AddBoolPtr(tx, "sampled", ev.Sampled)

	if ev.SpanCount.Dropped.Total != nil {
		s := common.MapStr{
			"dropped": common.MapStr{
				"total": ev.SpanCount.Dropped.Total,
			},
		}
		utility.AddCommonMapStr(tx, "span_count", s)
	}

	return tx
}

func (t *Event) Mappings(pa *payload) (time.Time, []utility.DocMapping) {
	mapping := []utility.DocMapping{
		{Key: "processor", Apply: func() common.MapStr {
			return common.MapStr{"name": processorName, "event": transactionDocType}
		}},
		{Key: transactionDocType, Apply: t.Transform},
		{Key: "context", Apply: func() common.MapStr { return t.Context }},
		{Key: "context.service", Apply: pa.Service.Transform},
		{Key: "context.system", Apply: pa.System.Transform},
		{Key: "context.process", Apply: pa.Process.Transform},
	}

	return t.Timestamp, mapping
}
