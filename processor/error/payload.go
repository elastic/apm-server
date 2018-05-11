package error

import (
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	transformations   = monitoring.NewInt(errorMetrics, "transformations")
	errorCounter      = monitoring.NewInt(errorMetrics, "errors")
	stacktraceCounter = monitoring.NewInt(errorMetrics, "stacktraces")
	frameCounter      = monitoring.NewInt(errorMetrics, "frames")
	processorEntry    = common.MapStr{"name": processorName, "event": errorDocType}
)

type Payload struct {
	Service m.Service
	System  *m.System
	Process *m.Process
	User    *m.User
	Events  []*Event
}

func DecodePayload(raw map[string]interface{}) ([]m.Transformable, error) {
	if raw == nil {
		return nil, nil
	}
	// pa := Payload{}

	// var err error
	// service, err := m.DecodeService(raw["service"], err)
	// if service != nil {
	// 	pa.Service = *service
	// }
	// pa.System, err = m.DecodeSystem(raw["system"], err)
	// pa.Process, err = m.DecodeProcess(raw["process"], err)
	// pa.User, err = m.DecodeUser(raw["user"], err)
	// if err != nil {
	// 	return nil, err
	// }

	decoder := utility.ManualDecoder{}
	errs := decoder.InterfaceArr(raw, "errors")
	transformables := make([]m.Transformable, len(errs))
	err := decoder.Err
	for idx, errData := range errs {
		transformables[idx], err = DecodeEvent(errData, err)
	}
	return transformables, err
}

// func (pa *Payload) Transform(conf config.Config) []beat.Event {
// 	transformations.Inc()
// 	logp.NewLogger("transform").Debugf("Transform error events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)
// 	errorCounter.Add(int64(len(pa.Events)))

// 	context := m.TransformContext{
// 		Service: &pa.Service,
// 		Process: pa.Process,
// 		System:  pa.System,
// 		User:    pa.User,
// 	}
