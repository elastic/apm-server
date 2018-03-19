package transaction

import (
	"errors"
	"time"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Event struct {
	Id        string
	Type      string
	Name      *string
	Result    *string
	Duration  float64
	Timestamp time.Time
	Context   common.MapStr
	Marks     common.MapStr
	Sampled   *bool
	SpanCount SpanCount
	Spans     []*Span
}
type SpanCount struct {
	Dropped Dropped
}
type Dropped struct {
	Total *int
}

func DecodeEvent(input interface{}, err error) (*Event, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for transaction event")
	}
	df := utility.DataFetcher{}
	e := Event{
		Id:        df.String(raw, "id"),
		Type:      df.String(raw, "type"),
		Name:      df.StringPtr(raw, "name"),
		Result:    df.StringPtr(raw, "result"),
		Duration:  df.Float64(raw, "duration"),
		Timestamp: df.TimeRFC3339(raw, "timestamp"),
		Context:   df.MapStr(raw, "context"),
		Marks:     df.MapStr(raw, "marks"),
		Sampled:   df.BoolPtr(raw, "sampled"),
		SpanCount: SpanCount{Dropped: Dropped{Total: df.IntPtr(raw, "total", "span_count", "dropped")}},
	}
	err = df.Err
	var span *Span
	spans := df.InterfaceArr(raw, "spans")
	e.Spans = make([]*Span, len(spans))
	for idx, sp := range spans {
		span, err = DecodeSpan(sp, err)
		e.Spans[idx] = span
	}
	return &e, err
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
