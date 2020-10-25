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

var (
	errTooManyTraceGroups = errors.New("too many trace groups")
	errNoMatchingPolicy   = errors.New("no matching policy")
)

// traceGroups maintains a collection of trace groups.
type traceGroups struct {
	// ingestRateDecayFactor is λ, the decay factor used for calculating the
	// exponentially weighted moving average ingest rate for each trace group.
	ingestRateDecayFactor float64

	// maxDynamicServices holds the maximum number of dynamic service groups
	// to maintain. Once this is reached, no new dynamic service groups will
	// be created, and events may be dropped.
	maxDynamicServices int

	// catchallServicePolicies, if non-nil, holds policies that apply to
	// services which are not explicitly specified.
	catchallServicePolicies []Policy

	mu            sync.RWMutex
	staticGroups  map[string]serviceGroups
	dynamicGroups map[string]serviceGroups
}

type serviceGroupKey struct {
	serviceEnvironment string
	traceOutcome       string
	traceName          string
}

func (k *serviceGroupKey) match(tx *model.Transaction) bool {
	if k.serviceEnvironment != "" && k.serviceEnvironment != tx.Metadata.Service.Environment {
		return false
	}
	if k.traceOutcome != "" && k.traceOutcome != tx.Outcome {
		return false
	}
	if k.traceName != "" && k.traceName != tx.Name {
		return false
	}
	return true
}

type serviceGroups []serviceGroup

type serviceGroup struct {
	key serviceGroupKey
	g   *traceGroup
}

// get returns the traceGroup to which tx should be added based on the
// defined sampling policies, matching policies in the order given.
func (sgs serviceGroups) get(tx *model.Transaction) (*traceGroup, bool) {
	for _, sg := range sgs {
		if sg.key.match(tx) {
			return sg.g, true
		}
	}
	return nil, false
}

func newTraceGroups(
	policies []Policy,
	maxDynamicServices int,
	ingestRateDecayFactor float64,
) *traceGroups {
	groups := &traceGroups{
		ingestRateDecayFactor: ingestRateDecayFactor,
		maxDynamicServices:    maxDynamicServices,
		staticGroups:          make(map[string]serviceGroups),
		dynamicGroups:         make(map[string]serviceGroups),
	}
	for _, policy := range policies {
		if policy.ServiceName == "" {
			// ServiceName is a special case; see PolicyCriteria.
			//
			// We maintain policies which are not service-specific separately, so we
			// can easily keep track how many dynamic services (dynamicGroups) there
			// are to enforce a limit, and to uphold the invariant that sampling groups
			// are service-specific, similar to head-based sampling.
			groups.catchallServicePolicies = append(groups.catchallServicePolicies, policy)
			continue
		}
		serviceGroups := groups.staticGroups[policy.ServiceName]
		groups.staticGroups[policy.ServiceName] = updateServiceNameGroups(policy, serviceGroups)
	}
	return groups
}

func updateServiceNameGroups(policy Policy, groups serviceGroups) serviceGroups {
	return append(groups, serviceGroup{
		key: serviceGroupKey{
			serviceEnvironment: policy.ServiceEnvironment,
			traceName:          policy.TraceName,
			traceOutcome:       policy.TraceOutcome,
		},
		g: newTraceGroup(policy.SampleRate),
	})
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
func (g *traceGroups) sampleTrace(tx *model.Transaction) (bool, error) {
	byService, ok := g.staticGroups[tx.Metadata.Service.Name]
	if !ok {
		// No static group, look for or create a dynamic group
		// if there are any catch-all policies defined.
		if len(g.catchallServicePolicies) == 0 {
			return false, errNoMatchingPolicy
		}
		// First attempt to locate a dynamic group with a read lock, to
		// avoid contention in the common case that a group has already
		// been defined.
		g.mu.RLock()
		byService, ok = g.dynamicGroups[tx.Metadata.Service.Name]
		if ok {
			defer g.mu.RUnlock()
		} else {
			g.mu.RUnlock()
			g.mu.Lock()
			defer g.mu.Unlock()
			byService, ok = g.dynamicGroups[tx.Metadata.Service.Name]
			if !ok {
				if len(g.dynamicGroups) == g.maxDynamicServices {
					return false, errTooManyTraceGroups
				}
				byService = make(serviceGroups, 0, len(g.catchallServicePolicies))
				for _, policy := range g.catchallServicePolicies {
					byService = updateServiceNameGroups(policy, byService)
				}
				g.dynamicGroups[tx.Metadata.Service.Name] = byService
			}
		}
	}
	group, ok := byService.get(tx)
	if !ok {
		return false, errNoMatchingPolicy
	}
	return group.sampleTrace(tx)
}

func (g *traceGroup) sampleTrace(tx *model.Transaction) (bool, error) {
	if g.samplingFraction == 0 {
		return false, nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.total++
	return g.reservoir.Sample(tx.Duration, tx.TraceID), nil
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
	for _, byService := range g.staticGroups {
		traceIDs, _, _ = g.finalizeServiceSampledTraces(byService, traceIDs)
	}
	maxDynamicServicesReached := len(g.dynamicGroups) == g.maxDynamicServices
	for serviceName, byService := range g.dynamicGroups {
		var total int
		var allMinReservoirSize bool
		traceIDs, total, allMinReservoirSize = g.finalizeServiceSampledTraces(byService, traceIDs)
		if allMinReservoirSize {
			if maxDynamicServicesReached || total == 0 {
				delete(g.dynamicGroups, serviceName)
			}
		}
	}
	return traceIDs
}

func (g *traceGroups) finalizeServiceSampledTraces(
	byService serviceGroups,
	traceIDs []string,
) (_ []string, total int, allMinReservoirSize bool) {
	allMinReservoirSize = true
	for _, group := range byService {
		total += group.g.total
		traceIDs = group.g.finalizeSampledTraces(traceIDs, g.ingestRateDecayFactor)
		if size := group.g.reservoir.Size(); size > minReservoirSize {
			allMinReservoirSize = false
		}
	}
	return traceIDs, total, allMinReservoirSize
}

// finalizeSampledTraces appends the group's current trace IDs to traceIDs, and
// returns the extended slice. On return the groups' sampling reservoirs will be
// reset.
func (g *traceGroup) finalizeSampledTraces(traceIDs []string, ingestRateDecayFactor float64) []string {
	g.mu.Lock()
	defer g.mu.Unlock()

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
		// lowest weighted traces to limit to the desired total.
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
