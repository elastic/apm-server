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
)

// MergeLabels merges eventLabels onto commonLabels. This is used for
// combining event-specific labels onto (metadata) global labels.
//
// If commonLabels is non-nil, it is first cloned. If commonLabels
// is nil, then eventLabels is cloned.
func MergeLabels(commonLabels, eventLabels common.MapStr) common.MapStr {
	if commonLabels == nil {
		return eventLabels.Clone()
	}
	combinedLabels := commonLabels.Clone()
	for k, v := range eventLabels {
		combinedLabels[k] = v
	}
	return combinedLabels
}

// NormalizeLabelValues transforms the values in labels, replacing any
// instance of json.Number with libbeat/common.Float, and returning
// labels.
func NormalizeLabelValues(labels common.MapStr) common.MapStr {
	for k, v := range labels {
		switch v := v.(type) {
		case json.Number:
			if floatVal, err := v.Float64(); err == nil {
				labels[k] = common.Float(floatVal)
			}
		}
	}
	return labels
}
