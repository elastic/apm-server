// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package sampling

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/elastic/apm-server/model"
)

const minReservoirSize = 1000

var errTooManyTraceGroups = errors.New("too many trace groups")

// traceGroups maintains a collection of trace groups.
type traceGroups struct {
	// defaultSamplingFraction holds the default sampling fraction to
	// set for new trace groups. See traceGroup.samplingFraction.
	defaultSamplingFraction float64

	// ingestRateDecayFactor is λ, the decay factor used for calculating the
	// exponentially weighted moving average ingest rate for each trace group.
	ingestRateDecayFactor float64

	// maxGroups holds the maximum number of trace groups to maintain.
	// Once this is reached, all trace events will be sampled.
	maxGroups int

	mu sync.RWMutex

	// TODO(axw) enable the user to define specific trace groups, along with
	// a specific sampling rate. The "groups" field would then track all other
	// trace groups, up to a configured maximum, using the default sampling rate.
	groups map[traceGroupKey]*traceGroup
}

func newTraceGroups(
	maxGroups int,
	defaultSamplingFraction float64,
	ingestRateDecayFactor float64,
) *traceGroups {
	return &traceGroups{
		defaultSamplingFraction: defaultSamplingFraction,
		ingestRateDecayFactor:   ingestRateDecayFactor,
		maxGroups:               maxGroups,
		groups:                  make(map[traceGroupKey]*traceGroup),
	}
}

// traceGroup represents a single trace group, including a measurement of the
// observed ingest rate, a trace ID weighted random sampling reservoir.
type traceGroup struct {
	// samplingFraction holds the configured fraction of traces in this
	// trace group to sample, as a fraction in the range (0,1).
	samplingFraction float64

	mu sync.Mutex
	// reservoir holds a random sample of root transactions observed
	// for this trace group, weighted by duration.
	reservoir *weightedRandomSample
	// total holds the total number of root transactions observed for
	// this trace group, including those that are not added to the
	// reservoir. This is used to update ingestRate.
	total int
	// ingestRate holds the exponentially weighted moving average number
	// of root transactions observed for this trace group per tail
	// sampling interval. This is read and written only by the periodic
	// finalizeSampledTraces calls.
	ingestRate float64
}

type traceGroupKey struct {
	// TODO(axw) review grouping attributes
	serviceName        string
	transactionName    string
	transactionOutcome string
}

// sampleTrace will return true if the root transaction is admitted to
// the in-memory sampling reservoir, and false otherwise.
//
// If the transaction is not admitted due to the transaction group limit
// having been reached, sampleTrace will return errTooManyTraceGroups.
func (g *traceGroups) sampleTrace(tx *model.Transaction) (bool, error) {
	key := traceGroupKey{
		serviceName:        tx.Metadata.Service.Name,
		transactionName:    tx.Name,
		transactionOutcome: tx.Outcome,
	}

	// First attempt to locate the group with a read lock, to avoid
	// contention in the common case that a group has already been
	// defined.
	g.mu.RLock()
	group, ok := g.groups[key]
	if ok {
		defer g.mu.RUnlock()
		group.mu.Lock()
		defer group.mu.Unlock()
		group.total++
		return group.reservoir.Sample(tx.Duration, tx.TraceID), nil
	}
	g.mu.RUnlock()

	g.mu.Lock()
	defer g.mu.Unlock()
	group, ok = g.groups[key]
	if ok {
		// We've got a write lock on g.mu, no need to lock group too.
		group.total++
		return group.reservoir.Sample(tx.Duration, tx.TraceID), nil
	} else if len(g.groups) == g.maxGroups {
		return false, errTooManyTraceGroups
	}

	group = &traceGroup{
		samplingFraction: g.defaultSamplingFraction,
		total:            1,
		reservoir: newWeightedRandomSample(
			rand.New(rand.NewSource(time.Now().UnixNano())),
			minReservoirSize,
		),
	}
	group.reservoir.Sample(tx.Duration, tx.TraceID)
	g.groups[key] = group
	return true, nil
}

// finalizeSampledTraces locks the groups, appends their current trace IDs to
// traceIDs, and returns the extended slice. On return the groups' sampling
// reservoirs will be reset.
//
// If the maximum number of groups has been reached, then any groups with the
// minimum reservoir size (low ingest or sampling rate) may be removed. These
// groups may also be removed if they have seen no activity in this interval.
func (g *traceGroups) finalizeSampledTraces(traceIDs []string) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	maxGroupsReached := len(g.groups) == g.maxGroups
	for key, group := range g.groups {
		total := group.total
		traceIDs = group.finalizeSampledTraces(traceIDs, g.ingestRateDecayFactor)
		if n := group.reservoir.Size(); n == minReservoirSize {
			if total == 0 || maxGroupsReached {
				delete(g.groups, key)
			}
		}
	}
	return traceIDs
}

// finalizeSampledTraces appends the group's current trace IDs to traceIDs, and
// returns the extended slice. On return the groups' sampling reservoirs will be
// reset.
func (g *traceGroup) finalizeSampledTraces(traceIDs []string, ingestRateDecayFactor float64) []string {
	if g.ingestRate == 0 {
		g.ingestRate = float64(g.total)
	} else {
		g.ingestRate *= 1 - ingestRateDecayFactor
		g.ingestRate += ingestRateDecayFactor * float64(g.total)
	}
	desiredTotal := int(math.Round(g.samplingFraction * float64(g.total)))
	g.total = 0

	for n := g.reservoir.Len(); n > desiredTotal; n-- {
		// The reservoir is larger than the desired fraction of the
		// observed total number of traces in this interval. Pop the
		// lowest weighed traces to limit to the desired total.
		g.reservoir.Pop()
	}
	traceIDs = append(traceIDs, g.reservoir.Values()...)

	// Resize the reservoir, so that it can hold the desired fraction of
	// the observed ingest rate.
	newReservoirSize := int(math.Round(g.samplingFraction * g.ingestRate))
	if newReservoirSize < minReservoirSize {
		newReservoirSize = minReservoirSize
	}
	g.reservoir.Reset()
	g.reservoir.Resize(newReservoirSize)
	return traceIDs
}
