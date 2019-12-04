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

package otel

import (
	"context"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/transaction"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

// Consumer is a place holder for proper implementation of transforming open-telemetry to elastic APM conform data
// This needs to be moved to the processor package when implemented properly
type Consumer struct {
	Reporter        publish.Reporter
	TransformConfig transform.Config
}

// ConsumeTraceData is only a place holder right now, needs to be implemented.
func (c *Consumer) ConsumeTraceData(ctx context.Context, td consumerdata.TraceData) error {
	transformables := []transform.Transformable{}
	for _, traceData := range td.Spans {
		transformables = append(transformables, &transaction.Event{
			TraceId: fmt.Sprintf("%x", traceData.TraceId),
			Id:      fmt.Sprintf("%x", traceData.SpanId),
		})
	}
	transformContext := &transform.Context{
		RequestTime: time.Now(),
		Config:      c.TransformConfig,
		Metadata:    metadata.Metadata{},
	}

	return c.Reporter(ctx, publish.PendingReq{
		Transformables: transformables,
		Tcontext:       transformContext,
		Trace:          true,
	})
}
