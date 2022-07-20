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

// Label keys are sanitized, replacing the reserved characters '.', '*' and '"'
// with '_'. Null-valued labels are omitted. Labels must not be mutated, as it
// may be shared by multiple events; if sanitization is required, a new map
// will be returned.
func sanitizeLabels(labels common.MapStr) common.MapStr {
	cloned := false
	for k, v := range labels {
		if v == nil {
			if !cloned {
				labels = labels.Clone()
				cloned = true
			}
			delete(labels, k)
			continue
		}
		if k2 := sanitizeLabelKey(k); k != k2 {
			if !cloned {
				labels = labels.Clone()
				cloned = true
			}
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
