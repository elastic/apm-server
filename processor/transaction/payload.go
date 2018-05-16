package transaction

import (
	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	transformations    = monitoring.NewInt(transactionMetrics, "transformations")
	transactionCounter = monitoring.NewInt(transactionMetrics, "transactions")
	spanCounter        = monitoring.NewInt(transactionMetrics, "spans")
	stacktraceCounter  = monitoring.NewInt(transactionMetrics, "stacktraces")
	frameCounter       = monitoring.NewInt(transactionMetrics, "frames")

	processorTransEntry = common.MapStr{"name": processorName, "event": transactionDocType}
	processorSpanEntry  = common.MapStr{"name": processorName, "event": spanDocType}
)

type Payload struct {
	Service m.Service
	System  *m.System
	Process *m.Process
	User    *m.User
	Events  []*Event
	Spans   []*Span
}

func DecodePayload(raw map[string]interface{}) (*Payload, error) {
	if raw == nil {
		return nil, nil
	}
	pa := &Payload{}

	var err error
	service, err := m.DecodeService(raw["service"], err)
	if service != nil {
		pa.Service = *service
	}
	pa.System, err = m.DecodeSystem(raw["system"], err)
	pa.Process, err = m.DecodeProcess(raw["process"], err)
	pa.User, err = m.DecodeUser(raw["user"], err)
	if err != nil {
		return nil, err
	}

	decoder := utility.ManualDecoder{}
	if sp := decoder.InterfaceArr(raw, "spans"); len(sp) > 0 {
		spans := make([]*Span, len(sp))
		for idx, s := range sp {
			spans[idx], err = DecodeDtSpan(s, err)
		}
		pa.Spans = spans
	}

	txs := decoder.InterfaceArr(raw, "transactions")
	err = decoder.Err
	pa.Events = make([]*Event, len(txs))
	var spans []*Span
	for idx, tx := range txs {
		pa.Events[idx], spans, err = DecodeEvent(tx, err)
		pa.Spans = append(pa.Spans, spans...)
	}
	return pa, err
}

func (pa *Payload) Transform(conf config.Config) []beat.Event {
	transformations.Inc()
	transactionCounter.Add(int64(len(pa.Events)))
	spanCounter.Add(int64(len(pa.Spans)))
	logp.NewLogger("transaction").Debugf("Transform transaction events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)

	context := m.NewContext(&pa.Service, pa.Process, pa.System, pa.User)
	spanContext := NewSpanContext(&pa.Service)

	var events []beat.Event
	for idx := 0; idx < len(pa.Events); idx++ {
		event := pa.Events[idx]
		ev := beat.Event{
			Fields: common.MapStr{
				"processor":        processorTransEntry,
				transactionDocType: event.Transform(),
				"context":          context.Transform(event.Context),
			},
			Timestamp: event.Timestamp,
		}
		events = append(events, ev)
		pa.Events[idx] = nil
	}

	for spIdx := 0; spIdx < len(pa.Spans); spIdx++ {
		sp := pa.Spans[spIdx]
		if frames := len(sp.Stacktrace); frames > 0 {
			stacktraceCounter.Inc()
			frameCounter.Add(int64(frames))
		}
		fields := common.MapStr{
			"processor": processorSpanEntry,
			spanDocType: sp.Transform(conf, pa.Service),
			"context":   spanContext.Transform(sp.Context),
		}
		if sp.TransactionId != nil {
			fields["transaction"] = common.MapStr{"id": sp.TransactionId}
		}
		events = append(events, beat.Event{Fields: fields, Timestamp: sp.Timestamp})
		pa.Spans[spIdx] = nil
	}

	return events
}
