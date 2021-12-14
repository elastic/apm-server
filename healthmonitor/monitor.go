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

package healthmonitor

import (
	"context"
	"errors"
	"time"

	"github.com/elastic/apm-server/adaptive"
	"github.com/elastic/beats/v7/libbeat/logp"
)

type Monitor struct {
	applied    map[adaptive.DecisionType]time.Time
	cancelFunc func()

	metrics  []Metric
	interval time.Duration
	cooldown time.Duration

	started bool
}

// New creates a new metric monitor.
func New(interval, cooldown time.Duration, metrics ...Metric) (Monitor, error) {
	if interval.Seconds() < 0 {
		interval = 30 * time.Second
	}
	if cooldown.Seconds() < 60 {
		cooldown = 5 * time.Minute
	}
	return Monitor{
		interval: interval,
		cooldown: cooldown,
		metrics:  metrics,
		applied:  make(map[adaptive.DecisionType]time.Time),
	}, nil
}

// Run executes the run monitor in the background until Stop is called or the
// passed context is done.
func (m *Monitor) Run(ctx context.Context) (<-chan adaptive.Decision, error) {
	if m.started {
		return nil, errors.New("monitor already started")
	}
	m.started = true

	cancelCtx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel
	adaptiveChan := make(chan adaptive.Decision)

	go func() {
		ticker := time.NewTicker(m.interval)
		defer func() {
			close(adaptiveChan)
			cancel()
			ticker.Stop()
		}()

		logger := logp.NewLogger("adaptive_monitor")
		for {
			select {
			case <-ctx.Done():
				return
			case <-cancelCtx.Done():
				return
			case <-ticker.C:
			}

			for _, metric := range m.metrics {
				decision := metric.BreachedTarget()

				// No action to take
				if decision.Type == adaptive.DecisionNone {
					continue
				}

				// Don't act if the cooldown isn't exceeded.
				if ts, ok := m.applied[decision.Type]; ok {
					if ts.After(time.Now()) {
						logger.Debug("adaptive decision needed but not enough time has passed since last")
						continue
					}
				}

				logger.Debugf("taking adaptive action based on decision %+v", decision)
				adaptiveChan <- decision

				// Create a cooldown entry.
				m.applied[decision.Type] = time.Now().Add(m.cooldown)
			}
		}
	}()
	return adaptiveChan, nil
}

// Stop stops the running monitor.
func (m *Monitor) Stop(ctx context.Context) {
	select {
	case <-ctx.Done():
	default:
		m.cancelFunc()
	}
}
