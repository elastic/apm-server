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

package modeldecoderutil

import (
	"encoding/json"
	"strconv"

	"github.com/elastic/elastic-agent-libs/mapstr"

	"github.com/elastic/apm-server/internal/model"
)

// GlobalLabelsFrom populates the Labels and NumericLabels from global labels
// in the metadata object.
func GlobalLabelsFrom(from mapstr.M, to *model.APMEvent) {
	to.NumericLabels = make(model.NumericLabels)
	to.Labels = make(model.Labels)
	MergeLabels(from, to)
	to.MarkGlobalLabels()
}

// MergeLabels merges eventLabels into the APMEvent. This is used for
// combining event-specific labels onto (metadata) global labels.
//
// If eventLabels is non-nil, it is first cloned.
func MergeLabels(eventLabels mapstr.M, to *model.APMEvent) {
	if to.NumericLabels == nil {
		to.NumericLabels = make(model.NumericLabels)
	}
	if to.Labels == nil {
		to.Labels = make(model.Labels)
	}
	for k, v := range eventLabels {
		switch v := v.(type) {
		case string:
			to.Labels.Set(k, v)
		case bool:
			to.Labels.Set(k, strconv.FormatBool(v))
		case float64:
			to.NumericLabels.Set(k, v)
		case json.Number:
			if floatVal, err := v.Float64(); err == nil {
				to.NumericLabels.Set(k, floatVal)
			}
		}
	}
	if len(to.NumericLabels) == 0 {
		to.NumericLabels = nil
	}
	if len(to.Labels) == 0 {
		to.Labels = nil
	}
}

// NormalizeLabelValues transforms the values in labels, replacing any
// instance of json.Number with libbeat/common.Float, and returning
// labels.
func NormalizeLabelValues(labels mapstr.M) mapstr.M {
	for k, v := range labels {
		switch v := v.(type) {
		case json.Number:
			if floatVal, err := v.Float64(); err == nil {
				labels[k] = floatVal
			}
		}
	}
	return labels
}
