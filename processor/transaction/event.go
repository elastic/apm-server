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

func (e *Event) decode(input interface{}) error {
	raw, ok := input.(map[string]interface{})
	if raw == nil {
		return nil
	}
	if !ok {
		return errors.New("Invalid type for transaction event")
	}
	df := utility.DataFetcher{}
	e.Id = df.String(raw, "id")
	e.Type = df.String(raw, "type")
	e.Name = df.StringPtr(raw, "name")
	e.Result = df.StringPtr(raw, "result")
	e.Duration = df.Float64(raw, "duration")
	e.Timestamp = df.TimeRFC3339(raw, "timestamp")
	e.Context = df.MapStr(raw, "context")
	e.Marks = df.MapStr(raw, "marks")
	e.Sampled = df.BoolPtr(raw, "sampled")
	e.SpanCount = SpanCount{Dropped: Dropped{Total: df.IntPtr(raw, "total", "span_count", "dropped")}}
	if df.Err != nil {
		return df.Err
	}
	if spans := df.InterfaceArr(raw, "spans"); spans != nil {
		e.Spans = make([]*Span, len(spans))
		for idx, sp := range spans {
			span := Span{}
			if err := span.decode(sp); err != nil {
				return err
			}
			e.Spans[idx] = &span
		}
	}
	return df.Err
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
