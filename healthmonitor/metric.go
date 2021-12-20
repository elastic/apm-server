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
	"math"
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

	lowBulkC  uint64
	highBulkC uint64
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
		freeBulkN:  10,
	}
}

// BreachedTarget returns the adaptive sampling decision.
func (m *ModelIndexerMetric) BreachedTarget() adaptive.Decision {
	// TODO(marclop): This needs to be removed, only using it for debugging
	// purposes.
	logger := logp.NewLogger("model_indexer_monitor")
	defer logger.Infof("FreeBulk: %d, 429Rate: %0.2f%%",
		atomic.LoadUint64(&m.freeBulkN), m.esTooManyH.Mean(),
	)

	// TODO(marclop): downscaling the number of bulkers when 429 are received
	// may be a good strategy, especially if the number of bulk indexers has
	// been upscaled.
	if metric := m.esTooManyH.Mean() / float64(100); metric >= m.target {
		downPct := 0.15
		return adaptive.Decision{
			Type: adaptive.DecisionDownsample,
			// TODO(marclop): Come up with a better formula.
			Factor: downPct + metric - math.Min(m.target, downPct),
		}
	} else if atomic.LoadUint64(&m.freeBulkN) == 0 {
		// If the Elasticsearch 429 error rate is lower than the threshold and
		// the number of available bulk indexers is low, we want to increase the
		// number of bulk indexers by 20%, but only after consecutive readings.
		if atomic.LoadUint64(&m.lowBulkC) < 12 {
			atomic.AddUint64(&m.lowBulkC, 1)
			return adaptive.Decision{}
		}
		// Reset the counter.
		atomic.StoreUint64(&m.lowBulkC, 0)
		return adaptive.Decision{
			Type:   adaptive.DecisionIndexerUpscale,
			Factor: 0.1,
		}
	} else if c := atomic.LoadUint64(&m.freeBulkN); c >= 10 {
		// TODO(marclop): record total number of indexers and compare dynamic threshold.
		if atomic.LoadUint64(&m.highBulkC) < 24 {
			atomic.AddUint64(&m.highBulkC, 1)
			return adaptive.Decision{}
		}
		// Reset the counter.
		atomic.StoreUint64(&m.highBulkC, 0)
		return adaptive.Decision{
			Type:   adaptive.DecisionIndexerDownscale,
			Factor: 0.1,
		}
	}
	// Reset the consecutive counters.
	atomic.StoreUint64(&m.lowBulkC, 0)
	atomic.StoreUint64(&m.highBulkC, 0)
	return adaptive.Decision{}
}

// Accumulate receives a stats structure and accumulates the 429/total ratio in
// the histogram.
func (m *ModelIndexerMetric) Accumulate(stats modelindexer.Stats) error {
	// Weigh the last measurement more heavily.
	atomic.StoreUint64(&m.freeBulkN,
		(atomic.LoadUint64(&m.freeBulkN)+(stats.Available*2))/3,
	)
	if stats.TooManyRequests > 0 && stats.Added > 0 {
		// If the accumulate rate is every 10s, 60 metrics will give us 10m
		// worth of data from the moment that TooManyRequests are recorded.
		if m.esTooManyH.TotalCount() > 60 {
			m.esTooManyH.Reset()
		}

		return m.esTooManyH.RecordValueAtomic(int64(
			float64(stats.TooManyRequests) / float64(stats.Added) * 100.0,
		))
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
