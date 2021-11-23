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
	"strings"

	"github.com/elastic/beats/v7/libbeat/common"
)

// Labels wraps a map[string]string or map[string][]string with utility
// methods.
type Labels map[string]LabelValue

// LabelValue wraps a `string` or `[]slice` to be set as a value for a key.
// Only one should be set, in cases where both are set, the `Values` field will
// be used and `Value` will be ignored.
type LabelValue struct {
	// Holds the label `string` value.
	Value string
	// Holds the label `[]string` value.
	Values []string
}

func (l Labels) Set(k string, v string) {
	l[k] = LabelValue{Value: v}
}

func (l Labels) SetSlice(k string, v []string) {
	l[k] = LabelValue{Values: v}
}

// Clone creates a deep copy of Labels.
func (l Labels) Clone() Labels {
	cp := make(Labels)
	for k, v := range l {
		to := LabelValue{Value: v.Value}
		if len(v.Values) > 0 {
			to.Values = make([]string, len(v.Values))
			copy(to.Values, v.Values)
		}
		cp[k] = to
	}
	return cp
}

func (l Labels) fields() common.MapStr {
	result := common.MapStr{}
	for k, v := range l {
		if v.Values != nil {
			result[k] = v.Values
		} else {
			result[k] = v.Value
		}
	}
	return sanitizeLabels(result)
}

// NumericLabels wraps a map[string]float64 or map[string][]float64 with utility
// methods.
type NumericLabels map[string]NumericLabelValue

// NumericLabelValue wraps a `float64` or `[]float64` to be set as a value for a
// key. Only one should be set, in cases where both are set, the `Values` field
// will be used and `Value` will be ignored.
type NumericLabelValue struct {
	// Holds the label `[]float64` value.
	Values []float64
	// Holds the label `float64` value.
	Value float64
}

func (l NumericLabels) Set(k string, v float64) {
	l[k] = NumericLabelValue{Value: v}
}

func (l NumericLabels) SetSlice(k string, v []float64) {
	l[k] = NumericLabelValue{Values: v}
}

// Clone creates a deep copy of NumericLabels.
func (l NumericLabels) Clone() NumericLabels {
	cp := make(NumericLabels)
	for k, v := range l {
		to := NumericLabelValue{Value: v.Value}
		if len(v.Values) > 0 {
			to.Values = make([]float64, len(v.Values))
			copy(to.Values, v.Values)
		}
		cp[k] = to
	}
	return cp
}

func (l NumericLabels) fields() common.MapStr {
	result := common.MapStr{}
	for k, v := range l {
		if v.Values != nil {
			result[k] = v.Values
		} else {
			result[k] = v.Value
		}
	}
	return sanitizeLabels(result)
}

// Label keys are sanitized, replacing the reserved characters '.', '*' and '"'
// with '_'. Null-valued labels are omitted.
func sanitizeLabels(labels common.MapStr) common.MapStr {
	for k, v := range labels {
		if v == nil {
			delete(labels, k)
			continue
		}
		if k2 := sanitizeLabelKey(k); k != k2 {
			delete(labels, k)
			labels[k2] = v
		}
	}
	return labels
}

func sanitizeLabelKey(k string) string {
	return strings.Map(replaceReservedLabelKeyRune, k)
}

func replaceReservedLabelKeyRune(r rune) rune {
	switch r {
	case '.', '*', '"':
		return '_'
	}
	return r
}
