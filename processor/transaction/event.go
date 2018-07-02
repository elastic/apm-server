package transaction

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Event struct {
	// new interface
	ParentId *string
	TraceId  *string
	HexId    *string

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
}
type SpanCount struct {
	Dropped Dropped
}
type Dropped struct {
	Total *int
}

func DecodeEvent(input interface{}, err error) (*Event, []*Span, error) {
	if input == nil || err != nil {
		return nil, nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("Invalid type for transaction event")
	}
	decoder := utility.ManualDecoder{}
	e := Event{
		Id:        decoder.String(raw, "id"),
		Type:      decoder.String(raw, "type"),
		Name:      decoder.StringPtr(raw, "name"),
		Result:    decoder.StringPtr(raw, "result"),
		Duration:  decoder.Float64(raw, "duration"),
		Timestamp: decoder.TimeRFC3339WithDefault(raw, "timestamp"),
		Context:   decoder.MapStr(raw, "context"),
		Marks:     decoder.MapStr(raw, "marks"),
		Sampled:   decoder.BoolPtr(raw, "sampled"),
		SpanCount: SpanCount{Dropped: Dropped{Total: decoder.IntPtr(raw, "total", "span_count", "dropped")}},
	}

	// format of Id indicates single service or distributed tracing
	if utility.IsUUID(e.Id) {

		// single service tracing format
		// no further additional settings necessary for backwards compatibility

		// nested spans are only allowed for single service tracing
		err = decoder.Err
		var spans []*Span
		if sp := decoder.InterfaceArr(raw, "spans"); len(sp) > 0 {
			spans = make([]*Span, len(sp))
			var span *Span
			for idx, s := range sp {
				span, err = DecodeSpan(s, err)
				if err != nil {
					fmt.Println(err.Error())
					return nil, nil, err
				}
				span.Timestamp = e.Timestamp
				span.TransactionId = &e.Id

				spans[idx] = span
			}
		}
		return &e, spans, nil
	} else {

		// set new hexId
		dtId := e.Id
		e.HexId = &dtId
		e.ParentId = decoder.StringPtr(raw, "parent_id")
		traceId := decoder.String(raw, "trace_id")
		if decoder.Err != nil {
			return nil, nil, decoder.Err
		}
		e.TraceId = &traceId

		// ensure some backwards compatibility
		// - set Id to `traceId:hexId`
		//   for global uniqueness when queried from old UI
		e.Id = strings.Join([]string{traceId, dtId}, "-")
		return &e, nil, nil
	}
}

func (t *Event) Transform() common.MapStr {
	tx := common.MapStr{"id": t.Id}
	utility.Add(tx, "name", t.Name)
	utility.Add(tx, "duration", utility.MillisAsMicros(t.Duration))
	utility.Add(tx, "type", t.Type)
	utility.Add(tx, "result", t.Result)
	utility.Add(tx, "marks", t.Marks)
	utility.Add(tx, "hex_id", t.HexId)
	utility.Add(tx, "parent_id", t.ParentId)
	utility.Add(tx, "trace_id", t.TraceId)

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
