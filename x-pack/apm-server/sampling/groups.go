// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package sampling

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/elastic/apm-data/model/modelpb"
)

const minReservoirSize = 1000

var (
	errTooManyTraceGroups = errors.New("too many trace groups")
	errNoMatchingPolicy   = errors.New("no matching policy")
)

// traceGroups maintains a collection of trace groups.
type traceGroups struct {
	// ingestRateDecayFactor is λ, the decay factor used for calculating the
	// exponentially weighted moving average ingest rate for each trace group.
	ingestRateDecayFactor float64

	// maxDynamicServiceGroups holds the maximum number of dynamic service groups
	// to maintain. Once this is reached, no new dynamic service groups will
	// be created, and events may be dropped.
	maxDynamicServiceGroups int

	// numDynamicServiceGroupsCounter is used for reporting the current number
	// of dynamic service groups.
	numDynamicServiceGroupsCounter metric.Int64UpDownCounter

	mu                      sync.RWMutex
	policyGroups            []policyGroup
	numDynamicServiceGroups int
}

type policyGroup struct {
	policy  Policy
	g       *traceGroup            // nil for catch-all
	dynamic map[string]*traceGroup // nil for static
}

func (g *policyGroup) match(transactionEvent *modelpb.APMEvent) bool {
	if g.policy.ServiceName != "" && g.policy.ServiceName != transactionEvent.Service.Name {
		return false
	}
	if g.policy.ServiceEnvironment != "" && g.policy.ServiceEnvironment != transactionEvent.Service.Environment {
		return false
	}
	if g.policy.TraceOutcome != "" && g.policy.TraceOutcome != transactionEvent.Event.Outcome {
		return false
	}
	if g.policy.TraceName != "" && g.policy.TraceName != transactionEvent.Transaction.Name {
		return false
	}
	return true
}

func newTraceGroups(
	meter metric.Meter,
	policies []Policy,
	maxDynamicServiceGroups int,
	ingestRateDecayFactor float64,
) *traceGroups {
	numDynamicServiceGroupsCounter, _ := meter.Int64UpDownCounter("apm-server.sampling.tail.dynamic_service_groups")
	groups := &traceGroups{
		ingestRateDecayFactor:          ingestRateDecayFactor,
		maxDynamicServiceGroups:        maxDynamicServiceGroups,
		numDynamicServiceGroupsCounter: numDynamicServiceGroupsCounter,
		policyGroups:                   make([]policyGroup, len(policies)),
	}
	for i, policy := range policies {
		pg := policyGroup{policy: policy}
		if policy.ServiceName != "" {
			pg.g = newTraceGroup(policy.SampleRate)
		} else {
			pg.dynamic = make(map[string]*traceGroup)
		}
		groups.policyGroups[i] = pg
	}
	return groups
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

func newTraceGroup(samplingFraction float64) *traceGroup {
	return &traceGroup{
		samplingFraction: samplingFraction,
		reservoir: newWeightedRandomSample(
			rand.New(rand.NewSource(time.Now().UnixNano())),
			minReservoirSize,
		),
	}
}

// sampleTrace will return true if the root transaction is admitted to
// the in-memory sampling reservoir, and false otherwise.
//
// If the transaction is not admitted due to the transaction group limit
// having been reached, sampleTrace will return errTooManyTraceGroups.
func (g *traceGroups) sampleTrace(transactionEvent *modelpb.APMEvent) (bool, error) {
	group, err := g.getTraceGroup(transactionEvent)
	if err != nil {
		return false, err
	}
	return group.sampleTrace(transactionEvent)
}

func (g *traceGroups) getTraceGroup(transactionEvent *modelpb.APMEvent) (*traceGroup, error) {
	var pg *policyGroup
	for i := range g.policyGroups {
		if g.policyGroups[i].match(transactionEvent) {
			pg = &g.policyGroups[i]
			break
		}
	}
	if pg == nil {
		return nil, errNoMatchingPolicy
	}
	if pg.g != nil {
		return pg.g, nil
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	group, ok := pg.dynamic[transactionEvent.GetService().GetName()]
	if !ok {
		if g.numDynamicServiceGroups == g.maxDynamicServiceGroups {
			return nil, errTooManyTraceGroups
		}
		g.numDynamicServiceGroups++
		g.numDynamicServiceGroupsCounter.Add(context.Background(), 1)
		group = newTraceGroup(pg.policy.SampleRate)
		pg.dynamic[transactionEvent.GetService().GetName()] = group
	}
	return group, nil
}

func (g *traceGroup) sampleTrace(transactionEvent *modelpb.APMEvent) (bool, error) {
	if g.samplingFraction == 0 {
		return false, nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.total++
	return g.reservoir.Sample(
		time.Duration(transactionEvent.GetEvent().GetDuration()).Seconds(),
		transactionEvent.GetTrace().GetId(),
	), nil
}

// finalizeSampledTraces locks the groups, appends their current trace IDs to
// traceIDs, and returns the extended slice. On return the groups' sampling
// reservoirs will be reset.
//
// If the maximum number of groups has been reached, then any dynamically
// created groups with the minimum reservoir size (low ingest or sampling rate)
// may be removed. These groups may also be removed if they have seen no
// activity in this interval.
func (g *traceGroups) finalizeSampledTraces(traceIDs []string) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	maxDynamicServiceGroupsReached := g.numDynamicServiceGroups == g.maxDynamicServiceGroups
	for _, pg := range g.policyGroups {
		if pg.g != nil {
			_, traceIDs = pg.g.finalizeSampledTraces(traceIDs, g.ingestRateDecayFactor)
			continue
		}
		for serviceName, group := range pg.dynamic {
			var total int
			total, traceIDs = group.finalizeSampledTraces(traceIDs, g.ingestRateDecayFactor)
			if (maxDynamicServiceGroupsReached || total == 0) && group.reservoir.Size() == minReservoirSize {
				g.numDynamicServiceGroups--
				g.numDynamicServiceGroupsCounter.Add(context.Background(), -1)
				delete(pg.dynamic, serviceName)
			}
		}
	}
	return traceIDs
}

// finalizeSampledTraces appends the group's current trace IDs to traceIDs, and
// returns total of the group and the extended slice.
// On return the groups' sampling reservoirs will be reset.
func (g *traceGroup) finalizeSampledTraces(traceIDs []string, ingestRateDecayFactor float64) (int, []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	oldTotal := g.total

	if g.ingestRate == 0 {
		g.ingestRate = float64(g.total)
	} else {
		g.ingestRate *= 1 - ingestRateDecayFactor
		g.ingestRate += ingestRateDecayFactor * float64(g.total)
	}
	desiredTotal := int(math.Ceil(g.samplingFraction * float64(g.total)))
	g.total = 0

	for n := g.reservoir.Len(); n > desiredTotal; n-- {
		// The reservoir is larger than the desired fraction of the
		// observed total number of traces in this interval. Pop the
		// lowest weighted traces to limit to the desired total.
		g.reservoir.Pop()
	}
	traceIDs = append(traceIDs, g.reservoir.Values()...)

	// Resize the reservoir, so that it can hold the desired fraction of
	// the observed ingest rate.
	newReservoirSize := int(math.Ceil(g.samplingFraction * g.ingestRate))
	if newReservoirSize < minReservoirSize {
		newReservoirSize = minReservoirSize
	}
	g.reservoir.Reset()
	g.reservoir.Resize(newReservoirSize)
	return oldTotal, traceIDs
}
