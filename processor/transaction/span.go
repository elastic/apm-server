package transaction

import (
	"time"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Span struct {
	Id               *int                `json:"id"`
	Name             string              `json:"name"`
	Type             string              `json:"type"`
	Start            float64             `json:"start"`
	Duration         float64             `json:"duration"`
	StacktraceFrames []m.StacktraceFrame `json:"stacktrace"`
	Context          common.MapStr       `json:"context"`
	Parent           *int                `json:"parent"`
}

func (t *Span) DocType() string {
	return "span"
}

func (t *Span) Transform(transactionId string, app m.App) common.MapStr {
	enhancer := utility.NewMapStrEnhancer()
	tr := common.MapStr{}
	enhancer.Add(tr, "id", t.Id)
	enhancer.Add(tr, "transaction", common.MapStr{"id": transactionId})
	enhancer.Add(tr, "name", t.Name)
	enhancer.Add(tr, "type", t.Type)
	enhancer.Add(tr, "start", utility.MillisAsMicros(t.Start))
	enhancer.Add(tr, "duration", utility.MillisAsMicros(t.Duration))
	enhancer.Add(tr, "parent", t.Parent)

	st := m.TransformStacktrace(t.StacktraceFrames, app)
	if len(st) > 0 {
		enhancer.Add(tr, "stacktrace", st)
	}
	return tr
}

func (t *Span) Mappings(pa *payload, tx Event) (time.Time, []m.DocMapping) {
	return tx.Timestamp,
		[]m.DocMapping{
			{Key: "processor", Apply: func() common.MapStr {
				return common.MapStr{"name": processorName, "event": t.DocType()}
			}},
			{Key: t.DocType(), Apply: func() common.MapStr { return t.Transform(tx.Id, pa.App) }},
			{Key: "context", Apply: func() common.MapStr { return t.Context }},
			{Key: "context.app", Apply: pa.App.MinimalTransform},
		}
}
