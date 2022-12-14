// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package labels

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-data/model"
)

func TestLabelsEqual(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		a := model.Labels{
			"a": model.LabelValue{Value: "b"},
			"b": model.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": model.LabelValue{Values: []string{"1", "2", "3"}},
		}
		b := model.Labels{
			"a": model.LabelValue{Value: "b"},
			"b": model.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": model.LabelValue{Values: []string{"1", "2", "3"}, Value: "b"},
		}
		assert.True(t, equalLabels(a, b))
		assert.True(t, equalLabels(b, a))
	})
	t.Run("false-length", func(t *testing.T) {
		a := model.Labels{
			"a": model.LabelValue{Value: "b"},
			"b": model.LabelValue{Values: []string{"b", "c", "d", "e"}},
		}
		b := model.Labels{
			"a": model.LabelValue{Value: "b"},
			"b": model.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": model.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, equalLabels(a, b))
		assert.False(t, equalLabels(b, a))
	})
	t.Run("false-elements-not-match", func(t *testing.T) {
		a := model.Labels{
			"a": model.LabelValue{Value: "b"},
			"b": model.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"d": model.LabelValue{Value: "c"},
		}
		b := model.Labels{
			"a": model.LabelValue{Value: "b"},
			"b": model.LabelValue{Values: []string{"e", "c", "d", "b"}},
			"c": model.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, equalLabels(a, b))
		assert.False(t, equalLabels(b, a))
	})
	t.Run("false-values-not-match", func(t *testing.T) {
		a := model.Labels{
			"a": model.LabelValue{Value: "c"},
			"b": model.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": model.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		b := model.Labels{
			"a": model.LabelValue{Value: "b"},
			"b": model.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": model.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, equalLabels(a, b))
		assert.False(t, equalLabels(b, a))
	})
	t.Run("false-slices-length-not-match", func(t *testing.T) {
		a := model.Labels{
			"a": model.LabelValue{Value: "b"},
			"b": model.LabelValue{Values: []string{"b", "c", "d"}},
			"c": model.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		b := model.Labels{
			"a": model.LabelValue{Value: "b"},
			"b": model.LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": model.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, equalLabels(a, b))
		assert.False(t, equalLabels(b, a))
	})
	t.Run("false-slices-elements-not-match", func(t *testing.T) {
		a := model.Labels{
			"a": model.LabelValue{Value: "b"},
			"b": model.LabelValue{Values: []string{"b", "c", "d", "a"}},
			"c": model.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		b := model.Labels{
			"a": model.LabelValue{Value: "b"},
			"b": model.LabelValue{Values: []string{"e", "c", "d", "b"}},
			"c": model.LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, equalLabels(a, b))
		assert.False(t, equalLabels(b, a))
	})
}

func TestNumericLabelsEqual(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		a := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": model.NumericLabelValue{Values: []float64{10, 20, 30}},
		}
		b := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": model.NumericLabelValue{Values: []float64{10, 20, 30}, Value: 1},
		}
		assert.True(t, equalNumericLabels(a, b))
		assert.True(t, equalNumericLabels(b, a))
	})
	t.Run("false-length", func(t *testing.T) {
		a := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
		}
		b := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": model.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, equalNumericLabels(a, b))
		assert.False(t, equalNumericLabels(b, a))
	})
	t.Run("false-elements-not-match", func(t *testing.T) {
		a := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"d": model.NumericLabelValue{Value: 2},
		}
		b := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": model.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, equalNumericLabels(a, b))
		assert.False(t, equalNumericLabels(b, a))
	})
	t.Run("false-values-not-match", func(t *testing.T) {
		a := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": model.NumericLabelValue{Value: 2},
		}
		b := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": model.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, equalNumericLabels(a, b))
		assert.False(t, equalNumericLabels(b, a))
	})
	t.Run("false-slices-length-not-match", func(t *testing.T) {
		a := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3}},
			"c": model.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		b := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": model.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, equalNumericLabels(a, b))
		assert.False(t, equalNumericLabels(b, a))
	})
	t.Run("false-slices-elements-not-match", func(t *testing.T) {
		a := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.5}},
			"c": model.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		b := model.NumericLabels{
			"a": model.NumericLabelValue{Value: 1},
			"b": model.NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": model.NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, equalNumericLabels(a, b))
		assert.False(t, equalNumericLabels(b, a))
	})
}
