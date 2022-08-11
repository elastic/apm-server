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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabelsEqual(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		a := Labels{
			"a": LabelValue{Value: "b"},
			"b": LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": LabelValue{Values: []string{"1", "2", "3"}},
		}
		b := Labels{
			"a": LabelValue{Value: "b"},
			"b": LabelValue{Values: []string{"e", "c", "d", "b"}},
			"c": LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.True(t, a.Equal(b))
		assert.True(t, b.Equal(a))
	})
	t.Run("false-length", func(t *testing.T) {
		a := Labels{
			"a": LabelValue{Value: "b"},
			"b": LabelValue{Values: []string{"b", "c", "d", "e"}},
		}
		b := Labels{
			"a": LabelValue{Value: "b"},
			"b": LabelValue{Values: []string{"e", "c", "d", "b"}},
			"c": LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, a.Equal(b))
		assert.False(t, b.Equal(a))
	})
	t.Run("false-elements-not-match", func(t *testing.T) {
		a := Labels{
			"a": LabelValue{Value: "b"},
			"b": LabelValue{Values: []string{"b", "c", "d", "e"}},
			"d": LabelValue{Value: "c"},
		}
		b := Labels{
			"a": LabelValue{Value: "b"},
			"b": LabelValue{Values: []string{"e", "c", "d", "b"}},
			"c": LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, a.Equal(b))
		assert.False(t, b.Equal(a))
	})
	t.Run("false-values-not-match", func(t *testing.T) {
		a := Labels{
			"a": LabelValue{Value: "c"},
			"b": LabelValue{Values: []string{"b", "c", "d", "e"}},
			"c": LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		b := Labels{
			"a": LabelValue{Value: "b"},
			"b": LabelValue{Values: []string{"e", "c", "d", "b"}},
			"c": LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, a.Equal(b))
		assert.False(t, b.Equal(a))
	})
	t.Run("false-slices-length-not-match", func(t *testing.T) {
		a := Labels{
			"a": LabelValue{Value: "b"},
			"b": LabelValue{Values: []string{"b", "c", "d"}},
			"c": LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		b := Labels{
			"a": LabelValue{Value: "b"},
			"b": LabelValue{Values: []string{"e", "c", "d", "b"}},
			"c": LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, a.Equal(b))
		assert.False(t, b.Equal(a))
	})
	t.Run("false-slices-elements-not-match", func(t *testing.T) {
		a := Labels{
			"a": LabelValue{Value: "b"},
			"b": LabelValue{Values: []string{"b", "c", "d", "a"}},
			"c": LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		b := Labels{
			"a": LabelValue{Value: "b"},
			"b": LabelValue{Values: []string{"e", "c", "d", "b"}},
			"c": LabelValue{Values: []string{"3", "2", "1"}, Value: "b"},
		}
		assert.False(t, a.Equal(b))
		assert.False(t, b.Equal(a))
	})
}

func TestNumericLabelsEqual(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		a := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": NumericLabelValue{Values: []float64{10, 20, 30}},
		}
		b := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.3, 1.2, 1.1, 1.4}},
			"c": NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.True(t, a.Equal(b))
		assert.True(t, b.Equal(a))
	})
	t.Run("false-length", func(t *testing.T) {
		a := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
		}
		b := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.3, 1.2, 1.1, 1.4}},
			"c": NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, a.Equal(b))
		assert.False(t, b.Equal(a))
	})
	t.Run("false-elements-not-match", func(t *testing.T) {
		a := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"d": NumericLabelValue{Value: 2},
		}
		b := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.3, 1.2, 1.1, 1.4}},
			"c": NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, a.Equal(b))
		assert.False(t, b.Equal(a))
	})
	t.Run("false-values-not-match", func(t *testing.T) {
		a := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.4}},
			"c": NumericLabelValue{Value: 2},
		}
		b := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.3, 1.2, 1.1, 1.4}},
			"c": NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, a.Equal(b))
		assert.False(t, b.Equal(a))
	})
	t.Run("false-slices-length-not-match", func(t *testing.T) {
		a := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.1, 1.2, 1.3}},
			"c": NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		b := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.3, 1.2, 1.1, 1.4}},
			"c": NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, a.Equal(b))
		assert.False(t, b.Equal(a))
	})
	t.Run("false-slices-elements-not-match", func(t *testing.T) {
		a := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.1, 1.2, 1.3, 1.5}},
			"c": NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		b := NumericLabels{
			"a": NumericLabelValue{Value: 1},
			"b": NumericLabelValue{Values: []float64{1.3, 1.2, 1.1, 1.4}},
			"c": NumericLabelValue{Values: []float64{30, 20, 10}, Value: 1},
		}
		assert.False(t, a.Equal(b))
		assert.False(t, b.Equal(a))
	})
}
