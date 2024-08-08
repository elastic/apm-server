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

package modelpb

// Labels wraps a map[string]string or map[string][]string with utility
// methods.
type Labels map[string]*LabelValue

// Set sets the label k to value v. If there existed a label in l with the same
// key, it will be replaced and its Global field will be set to false.
func (l Labels) Set(k string, v string) {
	l[k] = &LabelValue{Value: v}
}

// SetSlice sets the label k to value v. If there existed a label in l with the
// same key, it will be replaced and its Global field will be set to false.
func (l Labels) SetSlice(k string, v []string) {
	l[k] = &LabelValue{Values: v}
}

// Clone creates a deep copy of Labels.
func (l Labels) Clone() Labels {
	cp := make(Labels)
	for k, v := range l {
		to := LabelValue{Global: v.Global, Value: v.Value}
		if len(v.Values) > 0 {
			to.Values = make([]string, len(v.Values))
			copy(to.Values, v.Values)
		}
		cp[k] = &to
	}
	return cp
}

// NumericLabels wraps a map[string]float64 or map[string][]float64 with utility
// methods.
type NumericLabels map[string]*NumericLabelValue

// Set sets the label k to value v. If there existed a label in l with the same
// key, it will be replaced and its Global field will be set to false.
func (l NumericLabels) Set(k string, v float64) {
	l[k] = &NumericLabelValue{Value: v}
}

// SetSlice sets the label k to value v. If there existed a label in l with the
// same key, it will be replaced and its Global field will be set to false.
func (l NumericLabels) SetSlice(k string, v []float64) {
	l[k] = &NumericLabelValue{Values: v}
}

// Clone creates a deep copy of NumericLabels.
func (l NumericLabels) Clone() NumericLabels {
	cp := make(NumericLabels)
	for k, v := range l {
		to := NumericLabelValue{Global: v.Global, Value: v.Value}
		if len(v.Values) > 0 {
			to.Values = make([]float64, len(v.Values))
			copy(to.Values, v.Values)
		}
		cp[k] = &to
	}
	return cp
}
