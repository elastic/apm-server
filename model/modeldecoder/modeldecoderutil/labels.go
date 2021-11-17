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

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model"
)

// LabelsFrom populates the Labels and NumericLabels.
func LabelsFrom(from common.MapStr, to *model.APMEvent) {
	to.NumericLabels = filterLabels(NormalizeLabelValues(from.Clone()),
		filterNumberLabels,
	)
	to.Labels = filterLabels(from, filterStringLabels)
}

// MergeLabels merges eventLabels onto commonLabels. This is used for
// combining event-specific labels onto (metadata) global labels.
//
// If commonLabels is non-nil, it is first cloned. If commonLabels
// is nil, then eventLabels is cloned.
func MergeLabels(eventLabels common.MapStr, to *model.APMEvent) {
	to.NumericLabels = mergeLabels(to.NumericLabels, filterLabels(
		NormalizeLabelValues(eventLabels.Clone()),
		filterNumberLabels,
	))
	to.Labels = mergeLabels(to.Labels,
		filterLabels(eventLabels, filterStringLabels),
	)
}

func mergeLabels(commonLabels, eventLabels common.MapStr) common.MapStr {
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
				labels[k] = floatVal
			}
		}
	}
	return labels
}

func filterLabels(labels common.MapStr, fn func(interface{}) interface{}) common.MapStr {
	result := common.MapStr{}
	for k, v := range labels {
		if typeValue := fn(v); typeValue != nil {
			result[k] = typeValue
		}
	}
	if len(result) > 0 {
		return result
	}
	return nil
}

func filterStringLabels(v interface{}) interface{} {
	switch v := v.(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case []interface{}:
		res := make([]interface{}, 0, len(v))
		for i := range v {
			if val := filterStringLabels(v[i]); val != nil {
				res = append(res, val)
			}
		}
		return res
	}
	return nil
}

func filterNumberLabels(v interface{}) interface{} {
	switch v := v.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case int:
		return float64(v)
	case []interface{}:
		res := make([]interface{}, 0, len(v))
		for i := range v {
			if val := filterNumberLabels(v[i]); val != nil {
				res = append(res, val)
			}
		}
		return res
	}
	return nil
}
