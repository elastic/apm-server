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

package expvar

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Metric int

const (
	Bytes Metric = iota
	MemAllocs
	MemBytes
	ActiveEvents
	TotalEvents
	IntakeEventsAccepted
	IntakeEventsErrorsInvalid
	IntakeEventsErrorsTooLarge
	TransactionsProcessed
	SpansProcessed
	MetricsProcessed
	ErrorsProcessed
	NumGC
	RSSMemoryBytes
	Goroutines
	HeapAlloc
	HeapObjects
	AvailableBulkRequests
	ErrorElasticResponses
	ErrorOTLPTracesResponses
	ErrorOTLPMetricsResponses
	TBSLsmSize
	TBSVlogSize
)

type AggregateStats struct {
	First   int64
	Last    int64
	Min     int64
	Max     int64
	Mean    float64
	samples int64
}

type watchItem struct {
	notifyChan chan<- bool
	validator  func(int64) bool
}

// Collector defines a metric collector which queries expvar
// endpoint periodically and aggregates the collected metrics.
//
// Watch is a one-time hook into collector to know about when
// a specific state of a metric is observed by the collector.
type Collector struct {
	l       sync.Mutex
	metrics map[Metric]AggregateStats
	watches map[Metric]watchItem
	stopped bool

	logger *zap.Logger
}

// StartNewCollector creates a new collector and starts
// querying expvar endpoint at the specified interval.
func StartNewCollector(
	ctx context.Context,
	serverURL string,
	period time.Duration,
	logger *zap.Logger,
) (*Collector, error) {
	c := &Collector{
		metrics: make(map[Metric]AggregateStats),
		watches: make(map[Metric]watchItem),
		stopped: false,
		logger:  logger,
	}
	return c, c.start(ctx, serverURL, period)
}

// Get returns the aggregated stat of a given metric.
func (c *Collector) Get(m Metric) AggregateStats {
	c.l.Lock()
	defer c.l.Unlock()

	return c.metrics[m]
}

// Delta returns the diff of a metric value from the first
// event that was collected by the collector.
func (c *Collector) Delta(m Metric) int64 {
	c.l.Lock()
	defer c.l.Unlock()

	stats := c.metrics[m]
	return stats.Last - stats.First
}

// WatchMetric configures a new watch for a given metric. The
// watch is deleted after the specified expected value is
// observed or collector is stopped.
//
// WatchMetric returns a read only channel to observe the state
// of the watch. A true event on this channel refers to an
// observation with the expected value while the channel is
// closed to denote error in the collector. Both cases end the
// lifecycle of the watch.
func (c *Collector) WatchMetric(m Metric, expected int64) (<-chan bool, error) {
	c.l.Lock()
	defer c.l.Unlock()

	if c.stopped {
		return nil, errors.New("collector has stopped")
	}

	if _, ok := c.watches[m]; ok {
		return nil, errors.New("watch already exists for the given metric")
	}

	out := make(chan bool, 1)
	c.watches[m] = watchItem{
		notifyChan: out,
		validator:  func(in int64) bool { return in == expected },
	}

	return out, nil
}

func (c *Collector) accumulate(e expvar) {
	c.l.Lock()
	defer c.l.Unlock()

	c.processMetric(Goroutines, e.Goroutines)
	c.processMetric(Bytes, e.UncompressedBytes)
	c.processMetric(TotalEvents, e.TotalEvents)
	c.processMetric(ActiveEvents, e.ActiveEvents)
	c.processMetric(RSSMemoryBytes, e.RSSMemoryBytes)
	c.processMetric(AvailableBulkRequests, e.AvailableBulkRequests)
	c.processMetric(IntakeEventsAccepted, e.IntakeEventsAccepted)
	c.processMetric(IntakeEventsErrorsInvalid, e.IntakeEventsErrorsInvalid)
	c.processMetric(IntakeEventsErrorsTooLarge, e.IntakeEventsErrorsTooLarge)
	c.processMetric(TransactionsProcessed, e.TransactionsProcessed)
	c.processMetric(SpansProcessed, e.SpansProcessed)
	c.processMetric(MetricsProcessed, e.MetricsProcessed)
	c.processMetric(ErrorsProcessed, e.ErrorsProcessed)
	c.processMetric(ErrorElasticResponses, e.ErrorElasticResponses)
	c.processMetric(ErrorOTLPTracesResponses, e.ErrorOTLPTracesResponses)
	c.processMetric(ErrorOTLPMetricsResponses, e.ErrorOTLPMetricsResponses)
	c.processMetric(NumGC, int64(e.NumGC))
	c.processMetric(MemAllocs, int64(e.Mallocs))
	c.processMetric(MemBytes, int64(e.TotalAlloc))
	c.processMetric(HeapAlloc, int64(e.HeapAlloc))
	c.processMetric(HeapObjects, int64(e.HeapObjects))
	c.processMetric(TBSLsmSize, e.TBSLsmSize)
	c.processMetric(TBSVlogSize, e.TBSVlogSize)
}

func (c *Collector) processMetric(m Metric, val int64) {
	stats := c.metrics[m]
	c.metrics[m] = c.updateMetric(stats, val)

	if watch, ok := c.watches[m]; ok && watch.validator(val) {
		watch.notifyChan <- true
		close(watch.notifyChan)
		delete(c.watches, m)
	}
}

func (c *Collector) updateMetric(stats AggregateStats, value int64) AggregateStats {
	// Initialize for first value
	if stats.samples == 0 {
		stats.First, stats.Min, stats.Max = value, value, value
	}

	stats.samples++
	stats.Last = value
	stats.Mean += (float64(value) - stats.Mean) / float64(stats.samples)
	if stats.Min > value {
		stats.Min = value
	}
	if stats.Max < value {
		stats.Max = value
	}

	return stats
}

func (c *Collector) cleanup() {
	c.l.Lock()
	defer c.l.Unlock()

	c.stopped = true

	for m, watch := range c.watches {
		close(watch.notifyChan)
		delete(c.watches, m)
	}
}

func (c *Collector) start(ctx context.Context, serverURL string, period time.Duration) error {
	var first expvar
	err := queryExpvar(ctx, &first, serverURL)
	if err != nil {
		c.cleanup()
		return err
	}
	c.accumulate(first)

	outChan, errChan := run(ctx, serverURL, period)
	go func() {
		defer c.cleanup()

		for {
			select {
			case <-ctx.Done():
				return
			case err := <-errChan:
				c.logger.Warn("failed while querying expvar", zap.Error(err))
				return
			case m := <-outChan:
				c.accumulate(m)
			}
		}
	}()
	return nil
}

func run(ctx context.Context, serverURL string, period time.Duration) (<-chan expvar, <-chan error) {
	outChan := make(chan expvar)
	errChan := make(chan error)

	go func() {
		ticker := time.NewTicker(period)
		defer func() {
			ticker.Stop()
			close(outChan)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var e expvar
				ctxWithTimeout, cancel := context.WithTimeout(ctx, period+5*time.Second)
				err := queryExpvar(ctxWithTimeout, &e, serverURL)
				cancel()
				if err != nil {
					select {
					case errChan <- err:
					case <-ctx.Done():
					}
					return
				}
				select {
				case outChan <- e:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return outChan, errChan
}
