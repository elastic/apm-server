package transaction

import (
	"errors"
	"time"

	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

type Span struct {
	Id            *int
	Name          string
	Type          string
	Start         float64
	Duration      float64
	Context       common.MapStr
	Parent        *int
	Stacktrace    m.Stacktrace
	TransactionId *string
	Timestamp     time.Time
}

func DecodeSpan(input interface{}, err error) (*Span, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for span")
	}
	decoder := utility.ManualDecoder{}
	sp := Span{
		Id:            decoder.IntPtr(raw, "id"),
		Name:          decoder.String(raw, "name"),
		Type:          decoder.String(raw, "type"),
		Start:         decoder.Float64(raw, "start"),
		Duration:      decoder.Float64(raw, "duration"),
		Context:       decoder.MapStr(raw, "context"),
		Parent:        decoder.IntPtr(raw, "parent"),
		TransactionId: decoder.StringPtr(raw, "transaction_id"),
	}

	if sp.Context == nil {
		sp.Context = common.MapStr{}
	}

	if _, ok = raw["timestamp"]; ok {
		out := decoder.TimeRFC3339WithDefault(raw, "timestamp")
		sp.Timestamp = out
	}

	var stacktr *m.Stacktrace
	stacktr, err = m.DecodeStacktrace(raw["stacktrace"], decoder.Err)
	if stacktr != nil {
		sp.Stacktrace = *stacktr
	}
	return &sp, err
}

func (s *Span) Transform(config config.TransformConfig, context *m.TransformContext) beat.Event {
	spanCounter.Inc()

	tr := common.MapStr{}
	utility.Add(tr, "id", s.Id)
	utility.Add(tr, "name", s.Name)
	utility.Add(tr, "type", s.Type)
	utility.Add(tr, "start", utility.MillisAsMicros(s.Start))
	utility.Add(tr, "duration", utility.MillisAsMicros(s.Duration))
	utility.Add(tr, "parent", s.Parent)
	st := s.Stacktrace.Transform(config, context)
	utility.Add(tr, "stacktrace", st)

	addStacktraceCounter(s.Stacktrace)

	spanContext := s.Context
	utility.Add(spanContext, "service", context.Service.MinimalTransform())

	return beat.Event{
		Fields: common.MapStr{
			"processor":   processorSpanEntry,
			spanDocType:   tr,
			"transaction": common.MapStr{"id": s.TransactionId},
			"context":     spanContext,
		},
		Timestamp: s.Timestamp,
	}
}

func addStacktraceCounter(st m.Stacktrace) {
	if frames := len(st); frames > 0 {
		stacktraceCounter.Inc()
		frameCounter.Add(int64(frames))
	}
}
