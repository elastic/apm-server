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

package expvarmetrics

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Metric int

const (
	Goroutines Metric = iota
	Bytes
	MemAllocs
	MemBytes
	RSSMemoryBytes
	HeapAlloc
	HeapObjects
	NumGC
	PauseTotalNs
	AvailableBulkRequests
	TotalEvents
	ActiveEvents
	TransactionsProcessed
	SpansProcessed
	MetricsProcessed
	ErrorsProcessed
	ErrorElasticResponses
	ErrorOTLPTracesResponses
	ErrorOTLPMetricsResponses
)

type AggregateStats struct {
	First       int64
	Last        int64
	Min         int64
	Max         int64
	Mean        float64
	sampleCount int64
}

type watchItem struct {
	notifyChan chan<- bool
	validator  func(int64) bool
}

// Collector defines a metric collector which queries expvar
// endpoints and aggregates the collected metrics.
//
// Watchers are one-time hooks into collector to know about when
// the a specific state of the metric is observed by the collector.
type Collector struct {
	l         sync.Mutex
	serverURL string
	period    time.Duration
	metrics   map[Metric]*AggregateStats
	watchers  map[Metric]*watchItem
}

// StartNewCollector creates a new collector and starts
// querying expvar endpoint at the specified interval.
func StartNewCollector(ctx context.Context, serverURL string, period time.Duration) (*Collector, error) {
	c := &Collector{
		serverURL: serverURL,
		period:    period,
		metrics:   make(map[Metric]*AggregateStats),
		watchers:  make(map[Metric]*watchItem),
	}
	return c, c.start(ctx)
}

// Get returns the aggregated stat of a given metric.
func (c *Collector) Get(m Metric) AggregateStats {
	c.l.Lock()
	defer c.l.Unlock()

	stats := c.metrics[m]
	statsCopy := *stats
	return statsCopy
}

// Delta returns the diff of a metric valuefrom the first
// event that was collected by the collector.
func (c *Collector) Delta(m Metric) int64 {
	c.l.Lock()
	defer c.l.Unlock()

	stats := c.metrics[m]
	return stats.Last - stats.First
}

// AddWatch configures a new watcher for a given metric. The
// watcher is deleted after the specified expected value is
// observed.
//
// Watcher returns a read only channel to observe the state
// of the watch. A true event on this channel refers to an
// observation with the expected value while a false event
// refers to an error in the collector. Both cases end the
// lifecycle of a watcher.
func (c *Collector) AddWatch(m Metric, expected int64) (<-chan bool, error) {
	c.l.Lock()
	defer c.l.Unlock()

	if _, ok := c.watchers[m]; ok {
		return nil, errors.New("watcher already exists for the given metric")
	}

	out := make(chan bool)
	c.watchers[m] = &watchItem{
		notifyChan: out,
		validator:  func(in int64) bool { return in == expected },
	}

	return out, nil
}

func (c *Collector) addExpvar(e expvar) {
	c.l.Lock()
	defer c.l.Unlock()

	c.processMetric(Goroutines, e.Goroutines)
	c.processMetric(Bytes, e.UncompressedBytes)
	c.processMetric(TotalEvents, e.TotalEvents)
	c.processMetric(ActiveEvents, e.ActiveEvents)
	c.processMetric(RSSMemoryBytes, e.RSSMemoryBytes)
	c.processMetric(AvailableBulkRequests, e.AvailableBulkRequests)
	c.processMetric(TransactionsProcessed, e.TransactionsProcessed)
	c.processMetric(SpansProcessed, e.SpansProcessed)
	c.processMetric(MetricsProcessed, e.MetricsProcessed)
	c.processMetric(ErrorsProcessed, e.ErrorsProcessed)
	c.processMetric(ErrorElasticResponses, e.ErrorElasticResponses)
	c.processMetric(ErrorOTLPTracesResponses, e.ErrorOTLPTracesResponses)
	c.processMetric(ErrorOTLPMetricsResponses, e.ErrorOTLPMetricsResponses)
	c.processMetric(NumGC, int64(e.NumGC))
	c.processMetric(PauseTotalNs, int64(e.PauseTotalNs))
	c.processMetric(MemAllocs, int64(e.Mallocs))
	c.processMetric(MemBytes, int64(e.TotalAlloc))
	c.processMetric(HeapAlloc, int64(e.HeapAlloc))
	c.processMetric(HeapObjects, int64(e.HeapObjects))
}

func (c *Collector) processMetric(m Metric, val int64) {
	if _, ok := c.metrics[m]; !ok {
		c.metrics[m] = &AggregateStats{}
	}
	c.updateMetric(c.metrics[m], val)

	if watcher, ok := c.watchers[m]; ok && watcher.validator(val) {
		watcher.notifyChan <- true
		delete(c.watchers, m)
	}
}

func (c *Collector) updateMetric(stats *AggregateStats, value int64) {
	// Initialize for first value
	if stats.sampleCount == 0 {
		stats.First = value
		stats.Min = value
		stats.Max = value
	}

	stats.Last = value
	stats.sampleCount += 1
	stats.Mean += (float64(value) - stats.Mean) / float64(stats.sampleCount)
	if stats.Min > value {
		stats.Min = value
	}
	if stats.Max < value {
		stats.Max = value
	}
}

func (c *Collector) rejectWatchers() {
	c.l.Lock()
	defer c.l.Unlock()

	for _, watcher := range c.watchers {
		watcher.notifyChan <- false
	}
}

func (c *Collector) start(ctx context.Context) error {
	var first expvar
	err := queryExpvar(ctx, &first, c.serverURL)
	if err != nil {
		return err
	}
	c.addExpvar(first)

	outChan, errChan := run(ctx, c.serverURL, c.period)
	go func() {
		for {
			select {
			case <-ctx.Done():
				c.rejectWatchers()
				return
			case <-errChan:
				c.rejectWatchers()
				return
			case m := <-outChan:
				c.addExpvar(m)
			}
		}
	}()
	return nil
}

func run(ctx context.Context, serverURL string, period time.Duration) (<-chan expvar, <-chan error) {
	outChan := make(chan expvar)
	errChan := make(chan error)
	ticker := time.NewTicker(period)

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				close(outChan)
				return
			case <-ticker.C:
				var e expvar
				ctxWithTimeout, cancel := context.WithTimeout(ctx, period)
				err := queryExpvar(ctxWithTimeout, &e, serverURL)
				cancel()
				if err != nil {
					errChan <- err
					return
				}
				outChan <- e
			}
		}
	}()
	return outChan, errChan
}
