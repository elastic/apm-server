// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package hdrhistogram

import (
	"math"
	"math/rand"
	"testing"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerge(t *testing.T) {
	hist1, hist2 := getTestHistogram(), getTestHistogram()
	histRep1, histRep2 := New(), New()

	for i := 0; i < 1_000_000; i++ {
		v1, v2 := rand.Int63n(3_600_000_000), rand.Int63n(3_600_000_000)
		c1, c2 := rand.Int63n(1_000), rand.Int63n(1_000)
		hist1.RecordValues(v1, c1)
		histRep1.RecordValues(v1, c1)
		hist2.RecordValues(v2, c2)
		histRep2.RecordValues(v2, c2)
	}

	require.Equal(t, int64(0), hist1.Merge(hist2))
	histRep1.Merge(histRep2)
	assert.Empty(t, cmp.Diff(hist1.Export(), convertHistogramRepToSnapshot(histRep1)))
}

func TestBuckets(t *testing.T) {
	buckets := func(h *hdrhistogram.Histogram) (uint64, []uint64, []float64) {
		distribution := h.Distribution()
		counts := make([]uint64, 0, len(distribution))
		values := make([]float64, 0, len(distribution))

		var totalCount uint64
		for _, b := range distribution {
			if b.Count <= 0 {
				continue
			}
			count := uint64(math.Round(float64(b.Count) / histogramCountScale))
			counts = append(counts, count)
			values = append(values, float64(b.To))
			totalCount += count
		}
		return totalCount, counts, values
	}
	hist := getTestHistogram()
	histRep := New()

	recordValuesForAll := func(v, n int64) {
		hist.RecordValues(v, n)
		histRep.RecordValues(v, n)
	}

	// Explicitly test for recording values with 0 count
	recordValuesForAll(rand.Int63n(3_600_000_000), 0)
	for i := 0; i < 1_000_000; i++ {
		v := rand.Int63n(3_600_000_000)
		c := rand.Int63n(1_000)
		recordValuesForAll(v, c)
	}
	actualTotalCount, actualCounts, actualValues := histRep.Buckets()
	expectedTotalCount, expectedCounts, expectedValues := buckets(hist)

	assert.Equal(t, expectedTotalCount, actualTotalCount)
	assert.Equal(t, expectedCounts, actualCounts)
	assert.Equal(t, expectedValues, actualValues)
}

func getTestHistogram() *hdrhistogram.Histogram {
	return hdrhistogram.New(
		lowestTrackableValue,
		highestTrackableValue,
		int(significantFigures),
	)
}

func convertHistogramRepToSnapshot(h *HistogramRepresentation) *hdrhistogram.Snapshot {
	counts := make([]int64, countsLen)
	h.CountsRep.ForEach(func(bucket int32, value int64) {
		counts[bucket] += value
	})
	return &hdrhistogram.Snapshot{
		LowestTrackableValue:  h.LowestTrackableValue,
		HighestTrackableValue: h.HighestTrackableValue,
		SignificantFigures:    h.SignificantFigures,
		Counts:                counts,
	}
}
