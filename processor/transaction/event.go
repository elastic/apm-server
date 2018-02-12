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

func (t *Event) DocType() string {
	return "transaction"
}

func (t *Event) Transform() common.MapStr {
	enh := utility.NewMapStrEnhancer()
	tx := common.MapStr{"id": t.Id}
	enh.Add(tx, "name", t.Name)
	enh.Add(tx, "duration", utility.MillisAsMicros(t.Duration))
	enh.Add(tx, "type", t.Type)
	enh.Add(tx, "result", t.Result)
	enh.Add(tx, "marks", t.Marks)

	if t.Sampled == nil {
		enh.Add(tx, "sampled", true)
	} else {
		enh.Add(tx, "sampled", t.Sampled)
	}

	if t.SpanCount.Dropped.Total != nil {
		s := common.MapStr{
			"dropped": common.MapStr{
				"total": *t.SpanCount.Dropped.Total,
			},
		}
		enh.Add(tx, "span_count", s)
	}
	return tx
}

// This updates the event in place
func (t *Event) contextTransform(pa *payload) common.MapStr {
	if t.Context == nil {
		t.Context = make(map[string]interface{})
	}
	utility.InsertInMap(t.Context, "user", pa.User)
	return t.Context
}

func (t *Event) Mappings(pa *payload) (time.Time, []utility.DocMapping) {
	mapping := []utility.DocMapping{
		{Key: "processor", Apply: func() common.MapStr {
			return common.MapStr{"name": processorName, "event": t.DocType()}
		}},
		{Key: t.DocType(), Apply: t.Transform},
		{Key: "context", Apply: func() common.MapStr { return t.contextTransform(pa) }},
		{Key: "context.service", Apply: pa.Service.Transform},
		{Key: "context.system", Apply: pa.System.Transform},
		{Key: "context.process", Apply: pa.Process.Transform},
	}

	return t.Timestamp, mapping
}
