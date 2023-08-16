// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package labels

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-data/model/modelpb"
)

func TestLabelsEqual(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		a := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "b"},
			"b": &modelpb.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": &modelpb.LabelValue{Values: []string{"1", "2", "3"}},
		}
		b := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "b"},
			"b": &modelpb.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": &modelpb.LabelValue{Values: []string{"1", "2", "3"}, Value: "b"},
		}
		assert.True(t, equalLabels(a, b))
		assert.True(t, equalLabels(b, a))
	})
	t.Run("false-length", func(t *testing.T) {
		a := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "b"},
			"b": &modelpb.LabelValue{Values: []string{"b", "c", "d", "e"}},
		}
		b := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "b"},
			"b": &modelpb.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": &modelpb.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, equalLabels(a, b))
		assert.False(t, equalLabels(b, a))
	})
	t.Run("false-elements-not-match", func(t *testing.T) {
		a := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "b"},
			"b": &modelpb.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"d": &modelpb.LabelValue{Value: "c"},
		}
		b := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "b"},
			"b": &modelpb.LabelValue{Values: []string{"e", "c", "d", "b"}},
			"c": &modelpb.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, equalLabels(a, b))
		assert.False(t, equalLabels(b, a))
	})
	t.Run("false-values-not-match", func(t *testing.T) {
		a := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "c"},
			"b": &modelpb.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": &modelpb.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		b := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "b"},
			"b": &modelpb.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": &modelpb.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, equalLabels(a, b))
		assert.False(t, equalLabels(b, a))
	})
	t.Run("false-slices-length-not-match", func(t *testing.T) {
		a := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "b"},
			"b": &modelpb.LabelValue{Values: []string{"b", "c", "d"}},
			"c": &modelpb.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		b := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "b"},
			"b": &modelpb.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": &modelpb.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, equalLabels(a, b))
		assert.False(t, equalLabels(b, a))
	})
	t.Run("false-slices-elements-not-match", func(t *testing.T) {
		a := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "b"},
			"b": &modelpb.LabelValue{Values: []string{"b", "c", "d", "a"}},
			"c": &modelpb.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		b := modelpb.Labels{
			"a": &modelpb.LabelValue{Value: "b"},
			"b": &modelpb.LabelValue{Values: []string{"e", "c", "d", "b"}},
			"c": &modelpb.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, equalLabels(a, b))
		assert.False(t, equalLabels(b, a))
	})
}

func TestNumericLabelsEqual(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		a := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": &modelpb.NumericLabelValue{Values: []float64{10, 20, 30}},
		}
		b := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": &modelpb.NumericLabelValue{Values: []float64{10, 20, 30}, Value: 1},
		}
		assert.True(t, equalNumericLabels(a, b))
		assert.True(t, equalNumericLabels(b, a))
	})
	t.Run("false-length", func(t *testing.T) {
		a := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
		}
		b := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": &modelpb.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, equalNumericLabels(a, b))
		assert.False(t, equalNumericLabels(b, a))
	})
	t.Run("false-elements-not-match", func(t *testing.T) {
		a := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"d": &modelpb.NumericLabelValue{Value: 2},
		}
		b := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": &modelpb.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, equalNumericLabels(a, b))
		assert.False(t, equalNumericLabels(b, a))
	})
	t.Run("false-values-not-match", func(t *testing.T) {
		a := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": &modelpb.NumericLabelValue{Value: 2},
		}
		b := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": &modelpb.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, equalNumericLabels(a, b))
		assert.False(t, equalNumericLabels(b, a))
	})
	t.Run("false-slices-length-not-match", func(t *testing.T) {
		a := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3}},
			"c": &modelpb.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		b := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": &modelpb.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, equalNumericLabels(a, b))
		assert.False(t, equalNumericLabels(b, a))
	})
	t.Run("false-slices-elements-not-match", func(t *testing.T) {
		a := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.5}},
			"c": &modelpb.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		b := modelpb.NumericLabels{
			"a": &modelpb.NumericLabelValue{Value: 1},
			"b": &modelpb.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": &modelpb.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, equalNumericLabels(a, b))
		assert.False(t, equalNumericLabels(b, a))
	})
}
