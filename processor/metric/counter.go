package metric

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type counter struct {
	name  string
	count float64
	unit  *string
}

func (md *metricDecoder) decodeCounter(name string, raw map[string]interface{}) *counter {
	return &counter{
		name:  name,
		count: md.Float64(raw, "value"),
		unit:  md.StringPtr(raw, "unit"),
	}
}

func (c *counter) mapstr() common.MapStr {
	v := common.MapStr{
		"type":  "counter",
		"value": c.count,
	}
	utility.Add(v, "unit", c.unit)
	return v
}

func (c *counter) transform(m common.MapStr) error {
	m[c.name] = c.mapstr()
	return nil
}
