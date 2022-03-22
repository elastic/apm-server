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

package modelprocessor

import (
	"context"
	"sync"
	"sync/atomic"

	"go.elastic.co/apm"

	"github.com/elastic/apm-server/model"
)

// Limit the number of services recorded to 10000 to prevent memory exhaustion.
const serviceLimit = 10000

// ServiceCounter is a model.BatchProcessor that counts the number of events processed,
// partitioned by service.name and service.version.
type ServiceCounter struct {
	mu       sync.RWMutex
	services map[string]service
}

// service represents per-service name metrics for processed events.
type service struct {
	val    int64
	labels []apm.MetricLabel
}

// NewServiceCounter returns an ServiceCounter that counts events processed,
// paritioned by service.name and service.version.
func NewServiceCounter() *ServiceCounter {
	return &ServiceCounter{
		services: make(map[string]service),
	}
}

// GatherMetrics implements the MetricsGatherer interface.
func (c *ServiceCounter) GatherMetrics(ctx context.Context, m *apm.Metrics) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, s := range c.services {
		// TODO: Metric name?
		m.Add("processed_events", s.labels, float64(s.val))
	}
	return nil
}

// ProcessBatch counts events in b, grouping by APMEvent.Service.{Name,Version}.
func (c *ServiceCounter) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for _, event := range *b {
		if serviceName := event.Service.Name + event.Service.Version; serviceName != "" {
			c.mu.RLock()
			s, ok := c.services[serviceName]
			limit := len(c.services) >= serviceLimit
			c.mu.RUnlock()
			if !ok {
				if limit {
					// We've hit the service limit.
					// Continue processing in case we have
					// services that are already recorded
					// in c.services.
					continue
				}
				labels := []apm.MetricLabel{
					{Name: "service.name", Value: event.Service.Name},
					{Name: "service.version", Value: event.Service.Version},
				}
				s = service{val: 0, labels: labels}
			}
			atomic.AddInt64(&s.val, 1)
			c.mu.Lock()
			c.services[serviceName] = s
			c.mu.Unlock()
		}
	}
	return nil
}
