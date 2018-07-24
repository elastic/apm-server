// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package span

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
