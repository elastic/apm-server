package metric

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type gauge struct {
	name  string
	value float64
	unit  *string
}

func (md *metricDecoder) decodeGauge(name string, raw map[string]interface{}) *gauge {
	return &gauge{
		name:  name,
		value: md.Float64(raw, "value"),
		unit:  md.StringPtr(raw, "unit"),
	}
}

func (g *gauge) mapstr() common.MapStr {
	v := common.MapStr{
		"type":  "gauge",
		"value": g.value,
	}
	utility.Add(v, "unit", g.unit)
	return v
}

func (g *gauge) transform(m common.MapStr) error {
	m[g.name] = g.mapstr()
	return nil
}
