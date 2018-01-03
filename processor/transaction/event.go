package transaction

import (
	"time"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Event struct {
	Id        string
	Name      string
	Type      string
	Result    *string
	Duration  float64
	Timestamp time.Time
	Context   common.MapStr
	Spans     []Span
	Marks     common.MapStr
}

func (t *Event) DocType() string {
	return "transaction"
}

func (t *Event) Transform() common.MapStr {
	enh := utility.NewMapStrEnhancer()
	tx := common.MapStr{"id": t.Id}
	enh.Add(tx, "name", t.Name)
	enh.Add(tx, "duration", utility.MillisAsMicros(t.Duration))
	enh.Add(tx, "type", t.Type)
	enh.Add(tx, "result", t.Result)
	enh.Add(tx, "marks", t.Marks)
	return tx
}

func (t *Event) Mappings(pa *payload) (time.Time, []m.DocMapping) {
	return t.Timestamp,
		[]m.DocMapping{
			{Key: "processor", Apply: func() common.MapStr {
				return common.MapStr{"name": processorName, "event": t.DocType()}
			}},
			{Key: t.DocType(), Apply: t.Transform},
			{Key: "context", Apply: func() common.MapStr { return t.Context }},
			{Key: "context.service", Apply: pa.Service.Transform},
			{Key: "context.system", Apply: pa.System.Transform},
		}
}
