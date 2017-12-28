package stat

import (
	"time"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Event struct {
	Timestamp time.Time
	Values    common.MapStr
}

func (ev *Event) DocType() string {
	return "stat"
}

func (ev *Event) Transform() common.MapStr {
	enh := utility.NewMapStrEnhancer()

	tx := common.MapStr{}
	for key, val := range ev.Values {
		enh.Add(tx, key, val)
	}

	return tx
}

func (t *Event) Mappings(pa *payload) (time.Time, []utility.DocMapping) {
	mapping := []utility.DocMapping{
		{Key: "processor", Apply: func() common.MapStr {
			return common.MapStr{"name": processorName, "event": t.DocType()}
		}},
		{Key: t.DocType(), Apply: t.Transform},
		{Key: "context.service", Apply: pa.Service.Transform},
		{Key: "context.system", Apply: pa.System.Transform},
		{Key: "context.process", Apply: pa.Process.Transform},
	}

	return t.Timestamp, mapping
}
