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
	"encoding/json"
	"strings"

	"github.com/elastic/beats/v7/libbeat/common"
)

// If either or both globalLabels/eventLabels is non-empty, set a "labels"
// field in out to the combination of the labels.
//
// Label keys are sanitized, replacing the reserved characters '.', '*' and '"'
// with '_'. Event-specific labels take precedence over global labels.
// Null-valued labels are omitted.
func maybeSetLabels(out *mapStr, globalLabels, eventLabels common.MapStr) {
	n := len(globalLabels) + len(eventLabels)
	if n == 0 {
		return
	}
	combined := make(common.MapStr, n)
	for k, v := range globalLabels {
		if v == nil {
			continue
		}
		k := sanitizeLabelKey(k)
		combined[k] = normalizeLabelValue(v)
	}
	for k, v := range eventLabels {
		k := sanitizeLabelKey(k)
		if v == nil {
			delete(combined, k)
		} else {
			combined[k] = normalizeLabelValue(v)
		}
	}
	out.set("labels", combined)
}

// normalizeLabelValue transforms v into one of the accepted label value types:
// string, number, or boolean.
func normalizeLabelValue(v interface{}) interface{} {
	switch v := v.(type) {
	case json.Number:
		if floatVal, err := v.Float64(); err == nil {
			return common.Float(floatVal)
		}
	}
	return v // types are guaranteed by decoders
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
