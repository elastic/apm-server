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
	"sync/atomic"
	"time"

	"github.com/elastic/apm-server/adaptive"
	"github.com/elastic/apm-server/model/modelindexer"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/go-hdrhistogram"
)

type Metric interface {
	// BreachedTarget must return an `adaptive.Decision` when the metric
	// breaches its defined target threshold.
	BreachedTarget() adaptive.Decision
}

// ModelIndexerMetric can be used to monitor
type ModelIndexerMetric struct {
	esTooManyH *hdrhistogram.Histogram
	freeBulkN  uint64
	target     float64
}

// NewModelIndexerMetric creates a ModelIndexerRatioMetric with a target
// ratio of Indexer.TooManyRequests / Indexer.Added
func NewModelIndexerMetric(target float64) *ModelIndexerMetric {
	if target <= 0 {
		target = 0.1
	}
	return &ModelIndexerMetric{
		target:     target,
		esTooManyH: hdrhistogram.New(0, 100, 2),
		freeBulkN:  100,
	}
}

// BreachedTarget returns the adaptive sampling decision.
func (m *ModelIndexerMetric) BreachedTarget() adaptive.Decision {
	// TODO(marclop): using the median is quite arbitrary, test different
	// scenarios and metrics to see which one fits the bill better.
	metric := m.esTooManyH.Mean()
	if metric >= m.target {
		return adaptive.Decision{
			Type: adaptive.DecisionDownsample,
			// TODO(marclop): Come up with a good formula.
			Factor: 0.20 + metric - m.target,
		}
	} else if atomic.LoadUint64(&m.freeBulkN) < 2 {
		// If the Elasticsearch 429 error rate is lower than the threshold and
		// the number of available bulk indexers is low, we want to increase the
		// number of bulk indexers by 50%.
		return adaptive.Decision{
			Type:   adaptive.DecisionIndexerUpscale,
			Factor: 0.5,
		}
	}
	return adaptive.Decision{}
}

// Accumulate receives a stats structure and accumulates the 429/total ratio in
// the histogram.
func (m *ModelIndexerMetric) Accumulate(stats modelindexer.Stats) error {
	atomic.StoreUint64(&m.freeBulkN, stats.Available)
	// TODO(marclop): Determine how long to keep the history for, wouldn't be a
	// bad thing to reset the histogram after a certain amount of values have
	// been recorded.
	if stats.TooManyRequests > 0 && stats.Added > 0 {
		return m.esTooManyH.RecordValueAtomic(
			int64(stats.TooManyRequests / stats.Added * 100),
		)
	}
	return nil
}

// Monitor accumulates modelindexer.Indexer metrics on the specified period.
func (m *ModelIndexerMetric) Monitor(ctx context.Context, indexer *modelindexer.Indexer, interval time.Duration) {
	if interval.Seconds() < 10 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger := logp.NewLogger("model_indexer_metric")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if err := m.Accumulate(indexer.Stats()); err != nil {
			logger.Warnf("failed to accumulate modelindexer metrics: %s", err.Error())
		}
	}
}
