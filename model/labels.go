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
	"fmt"
	"strconv"
	"strings"

	"github.com/elastic/beats/v7/libbeat/common"
)

func filterLabels(labels common.MapStr, fn func(interface{}) interface{}) common.MapStr {
	result := common.MapStr{}
	for k, v := range labels {
		if typeValue := fn(v); typeValue != nil {
			if slice, ok := typeValue.([]interface{}); ok {
				for i, elem := range slice {
					result[fmt.Sprintf("%s_%d", k, i)] = elem
				}
			} else {
				result[k] = typeValue
			}
		}
	}
	return result
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
	case json.Number:
		if val, err := v.Float64(); err == nil {
			return val
		}
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
