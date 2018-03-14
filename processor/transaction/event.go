package transaction

import (
	"time"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Event struct {
	Id        string        `json:"id"`
	Name      *string       `json:"name"`
	Type      string        `json:"type"`
	Result    *string       `json:"result"`
	Duration  float64       `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	Context   common.MapStr `json:"context"`
	Spans     []*Span       `json:"spans"`
	Marks     common.MapStr `json:"marks"`
	Sampled   *bool         `json:"sampled"`
	SpanCount SpanCount     `json:"span_count"`
}
type SpanCount struct {
	Dropped Dropped `json:"dropped"`
}
type Dropped struct {
	Total *int `json:"total"`
}

func (t *Event) Transform() common.MapStr {
	tx := common.MapStr{"id": t.Id}
	utility.Add(tx, "name", t.Name)
	utility.Add(tx, "duration", utility.MillisAsMicros(t.Duration))
	utility.Add(tx, "type", t.Type)
	utility.Add(tx, "result", t.Result)
	utility.Add(tx, "marks", t.Marks)

	if t.Sampled == nil {
		utility.Add(tx, "sampled", true)
	} else {
		utility.Add(tx, "sampled", t.Sampled)
	}

	if t.SpanCount.Dropped.Total != nil {
		s := common.MapStr{
			"dropped": common.MapStr{
				"total": *t.SpanCount.Dropped.Total,
			},
		}
		utility.Add(tx, "span_count", s)
	}
	return tx
}
