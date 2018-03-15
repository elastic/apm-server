package transaction

import (
	"errors"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	transactionCounter  = monitoring.NewInt(transactionMetrics, "counter")
	spanCounter         = monitoring.NewInt(transactionMetrics, "spans")
	processorTransEntry = common.MapStr{"name": processorName, "event": transactionDocType}
	processorSpanEntry  = common.MapStr{"name": processorName, "event": spanDocType}
)

type payload struct {
	Service m.Service
	System  *m.System
	Process *m.Process
	User    *m.User
	Events  []Event
}

func (pa *payload) decode(raw map[string]interface{}) error {
	if err := pa.Service.Decode(raw["service"]); err != nil {
		return err
	}
	if pa.System == nil {
		pa.System = &m.System{}
	}
	if err := pa.System.Decode(raw["system"]); err != nil {
		return err
	}
	if pa.Process == nil {
		pa.Process = &m.Process{}
	}
	if err := pa.Process.Decode(raw["process"]); err != nil {
		return err
	}
	if pa.User == nil {
		pa.User = &m.User{}
	}
	if err := pa.User.Decode(raw["user"]); err != nil {
		return err
	}

	df := utility.DataFetcher{}
	if txs := df.InterfaceArr(raw, "transactions"); txs != nil {
		pa.Events = make([]Event, len(txs))
		for txIdx, tx := range txs {
			tx, ok := tx.(map[string]interface{})
			if !ok {
				return errors.New("Invalid type for transaction")
			}
			event := Event{}
			if err := event.decode(tx); err != nil {
				return err
			}
			pa.Events[txIdx] = event
		}
	} else {
		pa.Events = make([]Event, 0)
	}
	return df.Err
}

func (pa *payload) transform(config *pr.Config) []beat.Event {
	var events []beat.Event
	context := m.NewContext(&pa.Service, pa.Process, pa.System, pa.User)
	spanContext := NewSpanContext(&pa.Service)

	logp.NewLogger("transaction").Debugf("Transform transaction events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)

	transactionCounter.Add(int64(len(pa.Events)))
	for _, event := range pa.Events {

		ev := beat.Event{
			Fields: common.MapStr{
				"processor":        processorTransEntry,
				transactionDocType: event.Transform(),
				"context":          context.Transform(event.Context),
			},
			Timestamp: event.Timestamp,
		}
		events = append(events, ev)

		trId := common.MapStr{"id": event.Id}
		spanCounter.Add(int64(len(event.Spans)))
		for _, sp := range event.Spans {
			ev := beat.Event{
				Fields: common.MapStr{
					"processor":   processorSpanEntry,
					spanDocType:   sp.Transform(config, pa.Service),
					"transaction": trId,
					"context":     spanContext.Transform(sp.Context),
				},
				Timestamp: event.Timestamp,
			}
			events = append(events, ev)
		}
	}

	return events
}
