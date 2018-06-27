package metric

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type summary struct {
	name             string
	unit             *string
	count, sum       float64  // required
	max, min, stddev *float64 // optional
}

func (md *metricDecoder) decodeSummary(name string, raw map[string]interface{}) *summary {
	return &summary{
		name:   name,
		count:  md.Float64(raw, "count"),
		sum:    md.Float64(raw, "sum"),
		unit:   md.StringPtr(raw, "unit"),
		max:    md.Float64Ptr(raw, "max"),
		min:    md.Float64Ptr(raw, "min"),
		stddev: md.Float64Ptr(raw, "stddev"),
	}
}

func (s *summary) mapstr() common.MapStr {
	v := common.MapStr{
		"type":  "summary",
		"count": s.count,
		"sum":   s.sum,
	}
	utility.Add(v, "max", s.max)
	utility.Add(v, "min", s.min)
	utility.Add(v, "stddev", s.stddev)
	utility.Add(v, "unit", s.unit)
	return v
}

func (s *summary) transform(m common.MapStr) error {
	m[s.name] = s.mapstr()
	return nil
}
