// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// The MIT License (MIT)
//
// Copyright (c) 2014 Coda Hale
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package hdrhistogram provides an optimized histogram for sparse samples.
// This is a stop gap measure until we have [packed histogram implementation](https://www.javadoc.io/static/org.hdrhistogram/HdrHistogram/2.1.12/org/HdrHistogram/PackedHistogram.html).
package hdrhistogram

import (
	"fmt"
	"math"
	"math/bits"
	"slices"
	"time"
)

const (
	lowestTrackableValue  = 1
	highestTrackableValue = 3.6e+9 // 1 hour in microseconds
	significantFigures    = 2

	// We scale transaction counts in the histogram, which only permits storing
	// integer counts, to allow for fractional transactions due to sampling.
	//
	// e.g. if the sampling rate is 0.4, then each sampled transaction has a
	// representative count of 2.5 (1/0.4). If we receive two such transactions
	// we will record a count of 5000 (2 * 2.5 * histogramCountScale). When we
	// publish metrics, we will scale down to 5 (5000 / histogramCountScale).
	histogramCountScale = 1000
)

var (
	unitMagnitude               = getUnitMagnitude()
	bucketCount                 = getBucketCount()
	subBucketCount              = getSubBucketCount()
	subBucketHalfCountMagnitude = getSubBucketHalfCountMagnitude()
	subBucketHalfCount          = getSubBucketHalfCount()
	subBucketMask               = getSubBucketMask()
	countsLen                   = getCountsLen()
)

// HistogramRepresentation is an optimization over HDR histogram mainly useful
// for recording values clustered in some range rather than distributed over
// the full range of the HDR histogram. It is based on the [hdrhistogram-go](https://github.com/HdrHistogram/hdrhistogram-go) package.
// The package is not safe for concurrent usage, use an external lock
// protection if required.
type HistogramRepresentation struct {
	LowestTrackableValue  int64
	HighestTrackableValue int64
	SignificantFigures    int64
	CountsRep             HybridCountsRep
}

// New returns a new instance of HistogramRepresentation
func New() *HistogramRepresentation {
	return &HistogramRepresentation{
		LowestTrackableValue:  lowestTrackableValue,
		HighestTrackableValue: highestTrackableValue,
		SignificantFigures:    significantFigures,
	}
}

// RecordDuration records duration in the histogram representation. It
// supports recording float64 upto 3 decimal places. This is achieved
// by scaling the count.
func (h *HistogramRepresentation) RecordDuration(d time.Duration, n float64) error {
	count := int64(math.Round(n * histogramCountScale))
	v := d.Microseconds()

	return h.RecordValues(v, count)
}

// RecordValues records values in the histogram representation.
func (h *HistogramRepresentation) RecordValues(v, n int64) error {
	idx := h.countsIndexFor(v)
	if idx < 0 || int32(countsLen) <= idx {
		return fmt.Errorf("value %d is too large to be recorded", v)
	}
	h.CountsRep.Add(idx, n)
	return nil
}

// Merge merges the provided histogram representation.
// TODO: Add support for migration from a histogram representation
// with different parameters.
func (h *HistogramRepresentation) Merge(from *HistogramRepresentation) {
	if from == nil {
		return
	}
	from.CountsRep.ForEach(func(bucket int32, value int64) {
		h.CountsRep.Add(bucket, value)
	})
}

// Buckets converts the histogram into ordered slices of counts
// and values per bar along with the total count.
func (h *HistogramRepresentation) Buckets() (uint64, []uint64, []float64) {
	counts := make([]uint64, 0, h.CountsRep.Len())
	values := make([]float64, 0, h.CountsRep.Len())

	var totalCount uint64
	var prevBucket int32
	iter := h.iterator()
	iter.nextCountAtIdx()
	h.CountsRep.ForEach(func(bucket int32, scaledCounts int64) {
		if scaledCounts <= 0 {
			return
		}
		if iter.advance(int(bucket - prevBucket)) {
			count := uint64(math.Round(float64(scaledCounts) / histogramCountScale))
			counts = append(counts, count)
			values = append(values, float64(iter.highestEquivalentValue))
			totalCount += count
		}
		prevBucket = bucket
	})
	return totalCount, counts, values
}

func (h *HistogramRepresentation) countsIndexFor(v int64) int32 {
	bucketIdx := h.getBucketIndex(v)
	subBucketIdx := h.getSubBucketIdx(v, bucketIdx)
	return h.countsIndex(bucketIdx, subBucketIdx)
}

func (h *HistogramRepresentation) countsIndex(bucketIdx, subBucketIdx int32) int32 {
	baseBucketIdx := (bucketIdx + 1) << uint(subBucketHalfCountMagnitude)
	return baseBucketIdx + subBucketIdx - subBucketHalfCount
}

func (h *HistogramRepresentation) getBucketIndex(v int64) int32 {
	var pow2Ceiling = int64(64 - bits.LeadingZeros64(uint64(v|subBucketMask)))
	return int32(pow2Ceiling - int64(unitMagnitude) -
		int64(subBucketHalfCountMagnitude+1))
}

func (h *HistogramRepresentation) getSubBucketIdx(v int64, idx int32) int32 {
	return int32(v >> uint(int64(idx)+int64(unitMagnitude)))
}

func (h *HistogramRepresentation) valueFromIndex(bucketIdx, subBucketIdx int32) int64 {
	return int64(subBucketIdx) << uint(bucketIdx+unitMagnitude)
}

func (h *HistogramRepresentation) highestEquivalentValue(v int64) int64 {
	return h.nextNonEquivalentValue(v) - 1
}

func (h *HistogramRepresentation) nextNonEquivalentValue(v int64) int64 {
	bucketIdx := h.getBucketIndex(v)
	return h.lowestEquivalentValueGivenBucketIdx(v, bucketIdx) + h.sizeOfEquivalentValueRangeGivenBucketIdx(v, bucketIdx)
}

func (h *HistogramRepresentation) lowestEquivalentValueGivenBucketIdx(v int64, bucketIdx int32) int64 {
	subBucketIdx := h.getSubBucketIdx(v, bucketIdx)
	return h.valueFromIndex(bucketIdx, subBucketIdx)
}

func (h *HistogramRepresentation) sizeOfEquivalentValueRangeGivenBucketIdx(v int64, bucketIdx int32) int64 {
	subBucketIdx := h.getSubBucketIdx(v, bucketIdx)
	adjustedBucket := bucketIdx
	if subBucketIdx >= subBucketCount {
		adjustedBucket++
	}
	return int64(1) << uint(unitMagnitude+adjustedBucket)
}

func (h *HistogramRepresentation) iterator() *iterator {
	return &iterator{
		h:            h,
		subBucketIdx: -1,
	}
}

type iterator struct {
	h                       *HistogramRepresentation
	bucketIdx, subBucketIdx int32
	valueFromIdx            int64
	highestEquivalentValue  int64
}

// advance advances the iterator by count
func (i *iterator) advance(count int) bool {
	for c := 0; c < count; c++ {
		if !i.nextCountAtIdx() {
			return false
		}
	}
	i.highestEquivalentValue = i.h.highestEquivalentValue(i.valueFromIdx)
	return true
}

func (i *iterator) nextCountAtIdx() bool {
	// increment bucket
	i.subBucketIdx++
	if i.subBucketIdx >= subBucketCount {
		i.subBucketIdx = subBucketHalfCount
		i.bucketIdx++
	}

	if i.bucketIdx >= bucketCount {
		return false
	}

	i.valueFromIdx = i.h.valueFromIndex(i.bucketIdx, i.subBucketIdx)
	return true
}

func getSubBucketHalfCountMagnitude() int32 {
	largetValueWithSingleUnitResolution := 2 * math.Pow10(significantFigures)
	subBucketCountMagnitude := int32(math.Ceil(math.Log2(
		largetValueWithSingleUnitResolution,
	)))
	if subBucketCountMagnitude < 1 {
		return 0
	}
	return subBucketCountMagnitude - 1
}

func getUnitMagnitude() int32 {
	unitMag := int32(math.Floor(math.Log2(
		lowestTrackableValue,
	)))
	if unitMag < 0 {
		return 0
	}
	return unitMag
}

func getSubBucketCount() int32 {
	return int32(math.Pow(2, float64(getSubBucketHalfCountMagnitude()+1)))
}

func getSubBucketHalfCount() int32 {
	return getSubBucketCount() / 2
}

func getSubBucketMask() int64 {
	return int64(getSubBucketCount()-1) << uint(getUnitMagnitude())
}

func getCountsLen() int64 {
	return int64((getBucketCount() + 1) * (getSubBucketCount() / 2))
}

func getBucketCount() int32 {
	smallestUntrackableValue := int64(getSubBucketCount()) << uint(getUnitMagnitude())
	bucketsNeeded := int32(1)
	for smallestUntrackableValue < highestTrackableValue {
		if smallestUntrackableValue > (math.MaxInt64 / 2) {
			// next shift will overflow, meaning that bucket could
			// represent values up to ones greater than math.MaxInt64,
			// so it's the last bucket
			return bucketsNeeded + 1
		}
		smallestUntrackableValue <<= 1
		bucketsNeeded++
	}
	return bucketsNeeded
}

// bar represents a bar of histogram. Each bar has a bucket, representing
// where the bar belongs to in the histogram range, and the count of values
// in each bucket.
type bar struct {
	Bucket int32
	Count  int64
}

// HybridCountsRep represents a hybrid counts representation for
// sparse histogram. It is optimized to record a single value as
// integer type and more values as map.
type HybridCountsRep struct {
	bucket int32
	value  int64
	s      []bar
}

// Add adds a new value to a bucket of given index.
func (c *HybridCountsRep) Add(bucket int32, value int64) {
	if c.s == nil && c.bucket == 0 && c.value == 0 {
		c.bucket = bucket
		c.value = value
		return
	}
	if c.s == nil {
		// automatic promotion to slice
		c.s = make([]bar, 0, 128) // TODO: Use pool
		c.s = slices.Insert(c.s, 0, bar{Bucket: c.bucket, Count: c.value})
		c.bucket, c.value = 0, 0
	}
	at, found := slices.BinarySearchFunc(c.s, bar{Bucket: bucket}, compareBar)
	if found {
		c.s[at].Count += value
		return
	}
	c.s = slices.Insert(c.s, at, bar{Bucket: bucket, Count: value})
}

// ForEach iterates over each bucket and calls the given function.
func (c *HybridCountsRep) ForEach(f func(int32, int64)) {
	if c.s == nil && (c.bucket != 0 || c.value != 0) {
		f(c.bucket, c.value)
		return
	}
	for i := range c.s {
		f(c.s[i].Bucket, c.s[i].Count)
	}
}

// Len returns the number of buckets currently recording.
func (c *HybridCountsRep) Len() int {
	if c.s != nil {
		return len(c.s)
	}
	if c.bucket != 0 || c.value != 0 {
		return 1
	}
	return 0
}

// Get returns the count of values in a given bucket along with a bool
// which is false if the bucket is not found.
func (c *HybridCountsRep) Get(bucket int32) (int64, bool) {
	if c.s == nil {
		if c.bucket == bucket {
			return c.value, true
		}
		return 0, false
	}
	at, found := slices.BinarySearchFunc(c.s, bar{Bucket: bucket}, compareBar)
	if found {
		return c.s[at].Count, true
	}
	return 0, false
}

// Reset resets the values recorded.
func (c *HybridCountsRep) Reset() {
	c.bucket = 0
	c.value = 0
	c.s = c.s[:0]
}

// Equal returns true if same bucket and count is recorded in both.
func (c *HybridCountsRep) Equal(h *HybridCountsRep) bool {
	if c.Len() != h.Len() {
		return false
	}
	if c.Len() == 0 {
		return true
	}
	equal := true
	c.ForEach(func(bucket int32, value1 int64) {
		value2, ok := h.Get(bucket)
		if !ok || value1 != value2 {
			equal = false
		}
	})
	return equal
}

func compareBar(a, b bar) int {
	if a.Bucket == b.Bucket {
		return 0
	}
	if a.Bucket > b.Bucket {
		return 1
	}
	return -1
}
