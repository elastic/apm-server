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

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model"
)

// LabelsFrom populates the Labels and NumericLabels.
func LabelsFrom(from common.MapStr, to *model.APMEvent) {
	to.NumericLabels = make(model.NumericLabels)
	to.Labels = make(model.Labels)
	MergeLabels(from, to)
}

// MergeLabels merges eventLabels into the APMEvent. This is used for
// combining event-specific labels onto (metadata) global labels.
//
// If eventLabels is non-nil, it is first cloned.
func MergeLabels(eventLabels common.MapStr, to *model.APMEvent) {
	if to.NumericLabels == nil {
		to.NumericLabels = make(model.NumericLabels)
	}
	if to.Labels == nil {
		to.Labels = make(model.Labels)
	}
	for k, v := range NormalizeLabelValues(eventLabels.Clone()) {
		if !to.NumericLabels.MaybeSet(k, v) {
			to.Labels.MaybeSet(k, v)
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
func NormalizeLabelValues(labels common.MapStr) common.MapStr {
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
