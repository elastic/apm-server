// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package sampling

import (
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
)

func TestTraceGroupsPolicies(t *testing.T) {
	makeTransaction := func(serviceName, serviceEnvironment, traceOutcome, traceName string) *model.APMEvent {
		return &model.APMEvent{
			Service: model.Service{
				Name:        serviceName,
				Environment: serviceEnvironment,
			},
			Event: model.Event{
				Outcome: traceOutcome,
			},
			Processor: model.TransactionProcessor,
			Trace:     model.Trace{ID: uuid.Must(uuid.NewV4()).String()},
			Transaction: &model.Transaction{
				Name: traceName,
				ID:   uuid.Must(uuid.NewV4()).String(),
			},
		}
	}
	makePolicy := func(sampleRate float64, serviceName, serviceEnvironment, traceOutcome, traceName string) Policy {
		return Policy{
			SampleRate: sampleRate,
			PolicyCriteria: PolicyCriteria{
				ServiceName:        serviceName,
				ServiceEnvironment: serviceEnvironment,
				TraceName:          traceName,
				TraceOutcome:       traceOutcome,
			},
		}
	}

	const (
		staticServiceName  = "static-service"
		dynamicServiceName = "dynamic-service"
	)
	policies := []Policy{
		makePolicy(0.4, staticServiceName, "production", "error", "GET /healthcheck"),
		makePolicy(0.3, staticServiceName, "production", "error", ""),
		makePolicy(0.2, staticServiceName, "production", "", ""),
		makePolicy(0.1, staticServiceName, "", "", ""),
	}

	// Clone the policies without ServiceName set, so we have identical catch-all policies
	// that will match the dynamic service name.
	for _, policy := range policies[:] {
		policy.ServiceName = ""
		policies = append(policies, policy)
	}
	groups := newTraceGroups(policies, 1000, 1.0)

	assertSampleRate := func(sampleRate float64, serviceName, serviceEnvironment, traceOutcome, traceName string) {
		tx := makeTransaction(serviceName, serviceEnvironment, traceOutcome, traceName)
		const N = 1000
		for i := 0; i < N; i++ {
			if _, err := groups.sampleTrace(tx); err != nil {
				t.Fatal(err)
			}
		}
		sampled := groups.finalizeSampledTraces(nil)
		assert.Len(t, sampled, int(sampleRate*N))
	}

	for _, serviceName := range []string{staticServiceName, dynamicServiceName} {
		assertSampleRate(0.1, serviceName, "testing", "error", "GET /healthcheck")
		assertSampleRate(0.2, serviceName, "production", "success", "GET /healthcheck")
		assertSampleRate(0.3, serviceName, "production", "error", "GET /")
		assertSampleRate(0.4, serviceName, "production", "error", "GET /healthcheck")
	}
}

func TestTraceGroupsMax(t *testing.T) {
	const (
		maxDynamicServices    = 100
		ingestRateCoefficient = 1.0
	)
	policies := []Policy{{SampleRate: 1.0}}
	groups := newTraceGroups(policies, maxDynamicServices, ingestRateCoefficient)

	for i := 0; i < maxDynamicServices; i++ {
		serviceName := fmt.Sprintf("service_group_%d", i)
		for i := 0; i < minReservoirSize; i++ {
			admitted, err := groups.sampleTrace(&model.APMEvent{
				Service: model.Service{
					Name: serviceName,
				},
				Processor: model.TransactionProcessor,
				Trace:     model.Trace{ID: uuid.Must(uuid.NewV4()).String()},
				Transaction: &model.Transaction{
					Name: "whatever",
					ID:   uuid.Must(uuid.NewV4()).String(),
				},
			})
			require.NoError(t, err)
			assert.True(t, admitted)
		}
	}

	admitted, err := groups.sampleTrace(&model.APMEvent{
		Processor: model.TransactionProcessor,
		Trace:     model.Trace{ID: uuid.Must(uuid.NewV4()).String()},
		Transaction: &model.Transaction{
			Name: "overflow",
			ID:   uuid.Must(uuid.NewV4()).String(),
		},
	})
	assert.Equal(t, errTooManyTraceGroups, err)
	assert.False(t, admitted)
}

func TestTraceGroupReservoirResize(t *testing.T) {
	const (
		maxDynamicServices    = 1
		ingestRateCoefficient = 0.75
	)
	policies := []Policy{{SampleRate: 0.2}}
	groups := newTraceGroups(policies, maxDynamicServices, ingestRateCoefficient)

	sendTransactions := func(n int) {
		for i := 0; i < n; i++ {
			groups.sampleTrace(&model.APMEvent{
				Processor:   model.TransactionProcessor,
				Trace:       model.Trace{ID: "0102030405060708090a0b0c0d0e0f10"},
				Transaction: &model.Transaction{ID: "0102030405060708"},
			})
		}
	}

	// All groups start out with a reservoir size of 1000.
	sendTransactions(10000)
	assert.Len(t, groups.finalizeSampledTraces(nil), 1000) // initial reservoir size

	// We sent 10000 initially, and we send 20000 each subsequent iteration.
	// The number of sampled trace IDs will converge on 4000 (0.2*20000).
	for i, expected := range []int{
		2000, // 0.2 * 10000 (initial ingest rate)
		3500, // 0.2 * (0.25*10000 + 0.75*20000)
		3875, // 0.2 * (0.25*17500 + 0.75*20000)
		3969, // etc.
		3992,
		3998,
		4000,
		4000,
	} {
		sendTransactions(20000)
		assert.Len(t, groups.finalizeSampledTraces(nil), expected, fmt.Sprintf("iteration %d", i))
	}
}

func TestTraceGroupReservoirResizeMinimum(t *testing.T) {
	const (
		maxDynamicServices    = 1
		ingestRateCoefficient = 1.0
	)
	policies := []Policy{{SampleRate: 0.1}}
	groups := newTraceGroups(policies, maxDynamicServices, ingestRateCoefficient)

	sendTransactions := func(n int) {
		for i := 0; i < n; i++ {
			groups.sampleTrace(&model.APMEvent{
				Processor:   model.TransactionProcessor,
				Trace:       model.Trace{ID: "0102030405060708090a0b0c0d0e0f10"},
				Transaction: &model.Transaction{ID: "0102030405060708"},
			})
		}
	}

	sendTransactions(10000)
	assert.Len(t, groups.finalizeSampledTraces(nil), 1000) // initial reservoir size

	// The reservoir would normally be resized to fit the desired sampling
	// rate, but will never be resized to less than the minimum (1000).
	sendTransactions(1000)
	assert.Len(t, groups.finalizeSampledTraces(nil), 100)

	sendTransactions(10000)
	assert.Len(t, groups.finalizeSampledTraces(nil), 1000) // min reservoir size
}

func TestTraceGroupsRemoval(t *testing.T) {
	const (
		maxDynamicServices    = 2
		ingestRateCoefficient = 1.0
	)
	policies := []Policy{
		{PolicyCriteria: PolicyCriteria{ServiceName: "defined"}, SampleRate: 0.5},
		{SampleRate: 0.5},
		{PolicyCriteria: PolicyCriteria{ServiceName: "defined_later"}, SampleRate: 0.5},
	}
	groups := newTraceGroups(policies, maxDynamicServices, ingestRateCoefficient)

	for i := 0; i < 10000; i++ {
		_, err := groups.sampleTrace(&model.APMEvent{
			Service:     model.Service{Name: "many"},
			Processor:   model.TransactionProcessor,
			Transaction: &model.Transaction{},
		})
		assert.NoError(t, err)
	}
	_, err := groups.sampleTrace(&model.APMEvent{
		Service:     model.Service{Name: "few"},
		Processor:   model.TransactionProcessor,
		Transaction: &model.Transaction{},
	})
	assert.NoError(t, err)

	_, err = groups.sampleTrace(&model.APMEvent{
		Service:     model.Service{Name: "another"},
		Processor:   model.TransactionProcessor,
		Transaction: &model.Transaction{},
	})
	assert.Equal(t, errTooManyTraceGroups, err)

	// When there is a policy with an explicitly defined service name, that
	// will not be affected by the limit...
	_, err = groups.sampleTrace(&model.APMEvent{
		Service:     model.Service{Name: "defined"},
		Processor:   model.TransactionProcessor,
		Transaction: &model.Transaction{},
	})
	assert.NoError(t, err)

	// ...unless the policy with an explicitly defined service name comes after
	// a matching dynamic policy.
	_, err = groups.sampleTrace(&model.APMEvent{
		Service:     model.Service{Name: "defined_later"},
		Processor:   model.TransactionProcessor,
		Transaction: &model.Transaction{},
	})
	assert.Equal(t, errTooManyTraceGroups, err)

	// Finalizing should remove the "few" trace group, since its reservoir
	// size is at the minimum, and the number of groups is at the maximum.
	groups.finalizeSampledTraces(nil)

	// We should now be able to add another trace group.
	_, err = groups.sampleTrace(&model.APMEvent{
		Service:     model.Service{Name: "another"},
		Processor:   model.TransactionProcessor,
		Transaction: &model.Transaction{},
	})
	assert.NoError(t, err)
}

func BenchmarkTraceGroups(b *testing.B) {
	const (
		maxDynamicServices    = 1000
		ingestRateCoefficient = 1.0
	)
	policies := []Policy{{SampleRate: 1.0}}
	groups := newTraceGroups(policies, maxDynamicServices, ingestRateCoefficient)

	b.RunParallel(func(pb *testing.PB) {
		// Transaction identifiers are different for each goroutine, simulating
		// multiple agentss. This should demonstrate low contention.
		//
		// Duration is non-zero to ensure transactions have a non-zero chance of
		// being sampled.
		tx := model.APMEvent{
			Processor: model.TransactionProcessor,
			Event:     model.Event{Duration: time.Second},
			Transaction: &model.Transaction{
				Name: uuid.Must(uuid.NewV4()).String(),
			},
		}
		for pb.Next() {
			groups.sampleTrace(&tx)
			tx.Event.Duration += time.Second
		}
	})
}
