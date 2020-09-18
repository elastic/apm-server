// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package sampling

import (
	"fmt"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
)

func TestTraceGroups(t *testing.T) {
	const (
		maxGroups               = 100
		defaultSamplingFraction = 1.0
		ingestRateCoefficient   = 1.0
	)

	groups := newTraceGroups(maxGroups, defaultSamplingFraction, ingestRateCoefficient)
	for i := 0; i < maxGroups; i++ {
		transactionName := fmt.Sprintf("transaction_group_%d", i)
		for i := 0; i < minReservoirSize; i++ {
			admitted, err := groups.sampleTrace(&model.Transaction{
				Name:    transactionName,
				TraceID: uuid.Must(uuid.NewV4()).String(),
				ID:      uuid.Must(uuid.NewV4()).String(),
			})
			require.NoError(t, err)
			assert.True(t, admitted)
		}
	}

	admitted, err := groups.sampleTrace(&model.Transaction{
		Name:    "overflow",
		TraceID: uuid.Must(uuid.NewV4()).String(),
		ID:      uuid.Must(uuid.NewV4()).String(),
	})
	assert.Equal(t, errTooManyTraceGroups, err)
	assert.False(t, admitted)
}

func TestTraceGroupReservoirResize(t *testing.T) {
	const (
		maxGroups               = 1
		defaultSamplingFraction = 0.2
		ingestRateCoefficient   = 0.75
	)
	groups := newTraceGroups(maxGroups, defaultSamplingFraction, ingestRateCoefficient)

	sendTransactions := func(n int) {
		for i := 0; i < n; i++ {
			groups.sampleTrace(&model.Transaction{
				TraceID: "0102030405060708090a0b0c0d0e0f10",
				ID:      "0102030405060708",
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
		maxGroups               = 1
		defaultSamplingFraction = 0.1
		ingestRateCoefficient   = 1.0
	)
	groups := newTraceGroups(maxGroups, defaultSamplingFraction, ingestRateCoefficient)

	sendTransactions := func(n int) {
		for i := 0; i < n; i++ {
			groups.sampleTrace(&model.Transaction{
				TraceID: "0102030405060708090a0b0c0d0e0f10",
				ID:      "0102030405060708",
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
		maxGroups               = 2
		defaultSamplingFraction = 0.5
		ingestRateCoefficient   = 1.0
	)
	groups := newTraceGroups(maxGroups, defaultSamplingFraction, ingestRateCoefficient)

	for i := 0; i < 10000; i++ {
		_, err := groups.sampleTrace(&model.Transaction{Name: "many"})
		assert.NoError(t, err)
	}
	_, err := groups.sampleTrace(&model.Transaction{Name: "few"})
	assert.NoError(t, err)

	_, err = groups.sampleTrace(&model.Transaction{Name: "another"})
	assert.Equal(t, errTooManyTraceGroups, err)

	// Finalizing should remove the "few" trace group, since its reservoir
	// size is at the minimum, and the number of groups is at the maximum.
	groups.finalizeSampledTraces(nil)

	// We should now be able to add another trace group.
	_, err = groups.sampleTrace(&model.Transaction{Name: "another"})
	assert.NoError(t, err)
}

func BenchmarkTraceGroups(b *testing.B) {
	const (
		maxGroups               = 1000
		defaultSamplingFraction = 1.0
		ingestRateCoefficient   = 1.0
	)
	groups := newTraceGroups(maxGroups, defaultSamplingFraction, ingestRateCoefficient)

	b.RunParallel(func(pb *testing.PB) {
		// Transaction identifiers are different for each goroutine, simulating
		// multiple agentss. This should demonstrate low contention.
		//
		// Duration is non-zero to ensure transactions have a non-zero chance of
		// being sampled.
		tx := model.Transaction{
			Duration: 1000,
			Name:     uuid.Must(uuid.NewV4()).String(),
		}
		for pb.Next() {
			groups.sampleTrace(&tx)
			tx.Duration += 1000
		}
	})
}
