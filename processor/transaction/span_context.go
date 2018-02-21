package transaction

import (
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type SpanContext struct {
	service common.MapStr
}

func NewSpanContext(service *m.Service) *SpanContext {
	return &SpanContext{service: service.MinimalTransform()}
}

func (c *SpanContext) Transform(m common.MapStr) common.MapStr {
	if m == nil {
		m = common.MapStr{}
	} else {
		for k, v := range m {
			utility.Add(m, k, v)
		}
	}
	utility.Add(m, "service", c.service)
	if len(m) == 0 {
		return nil
	}
	return m
}
