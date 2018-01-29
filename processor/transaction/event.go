package transaction

import (
	"time"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Event struct {
	Id        string
	Name      *string
	Type      string
	Result    *string
	Duration  float64
	Timestamp time.Time
	Context   common.MapStr
	Spans     []Span
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

func (ev *Event) DocType() string {
	return "transaction"
}

func (ev *Event) Transform() common.MapStr {
	enh := utility.NewMapStrEnhancer()
	tx := common.MapStr{"id": ev.Id}
	enh.Add(tx, "name", ev.Name)
	enh.Add(tx, "duration", utility.MillisAsMicros(ev.Duration))
	enh.Add(tx, "type", ev.Type)
	enh.Add(tx, "result", ev.Result)
	enh.Add(tx, "marks", ev.Marks)

	if ev.Sampled == nil {
		enh.Add(tx, "sampled", true)
	} else {
		enh.Add(tx, "sampled", ev.Sampled)
	}

	if ev.SpanCount.Dropped.Total != nil {
		s := common.MapStr{
			"dropped": common.MapStr{
				"total": *ev.SpanCount.Dropped.Total,
			},
		}
		enh.Add(tx, "span_count", s)
	}
	return tx
}

func (t *Event) Mappings(pa *payload) (time.Time, []utility.DocMapping) {
	mapping := []utility.DocMapping{
		{Key: "processor", Apply: func() common.MapStr {
			return common.MapStr{"name": processorName, "event": t.DocType()}
		}},
		{Key: t.DocType(), Apply: t.Transform},
		{Key: "context", Apply: func() common.MapStr { return t.Context }},
		{Key: "context.service", Apply: pa.Service.Transform},
		{Key: "context.system", Apply: pa.System.Transform},
		{Key: "context.process", Apply: pa.Process.Transform},
	}

	return t.Timestamp, mapping
}
