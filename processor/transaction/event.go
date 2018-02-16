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
	Spans     []*Span
	Marks     common.MapStr
	Sampled   *bool
	SpanCount
}
type SpanCount struct {
	Dropped
}
type Dropped struct {
	DroppedTotal *int
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

	if t.SpanCount.Dropped.DroppedTotal != nil {
		s := common.MapStr{
			"dropped": common.MapStr{
				"total": *t.SpanCount.Dropped.DroppedTotal,
			},
		}
		utility.Add(tx, "span_count", s)
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
