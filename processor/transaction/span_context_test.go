package transaction

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/libbeat/common"
)

func TestSpanContext(t *testing.T) {
	tests := []struct {
		service *model.Service
		context *SpanContext
	}{
		{
			service: nil,
			context: &SpanContext{},
		},
		{
			service: &model.Service{},
			context: &SpanContext{
				service: common.MapStr{"name": "", "agent": common.MapStr{"version": "", "name": ""}},
			},
		},
		{
			service: &model.Service{Name: "service"},
			context: &SpanContext{
				service: common.MapStr{"name": "service", "agent": common.MapStr{"version": "", "name": ""}},
			},
		},
	}

	for idx, te := range tests {
		ctx := NewSpanContext(te.service)
		assert.Equal(t, te.context, ctx,
			fmt.Sprintf("<%v> Expected: %v, Actual: %v", idx, te.context, ctx))
	}
}

func TestSpanContextTransform(t *testing.T) {

	tests := []struct {
		context *SpanContext
		m       common.MapStr
		out     common.MapStr
	}{
		{
			context: &SpanContext{},
			m:       common.MapStr{},
			out:     common.MapStr{},
		},
		{
			context: &SpanContext{},
			m:       common.MapStr{"user": common.MapStr{"id": 123}},
			out:     common.MapStr{"user": common.MapStr{"id": 123}},
		},
		{
			context: &SpanContext{
				service: common.MapStr{"name": "service", "agent": common.MapStr{"version": "", "name": ""}},
			},
			m: common.MapStr{"foo": "bar", "user": common.MapStr{"id": 123, "username": "foo"}},
			out: common.MapStr{
				"foo":     "bar",
				"user":    common.MapStr{"id": 123, "username": "foo"},
				"service": common.MapStr{"name": "service", "agent": common.MapStr{"version": "", "name": ""}},
			},
		},
	}

	for idx, te := range tests {
		out := te.context.Transform(te.m)
		assert.Equal(t, te.out, out,
			fmt.Sprintf("<%v> Expected: %v, Actual: %v", idx, te.out, out))
	}
}
