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

package utility

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/elastic/beats/libbeat/common"
)

func Add(m common.MapStr, key string, val interface{}) {
	if m == nil || key == "" {
		return
	}

	if val == nil {
		delete(m, key)
		return
	}

	switch value := val.(type) {
	case *bool:
		if value != nil {
			m[key] = *value
		} else {
			delete(m, key)
		}
	case *int:
		if value != nil {
			m[key] = *value
		} else {
			delete(m, key)
		}
	case *int64:
		if newVal := val.(*int64); newVal != nil {
			m[key] = *newVal
		} else {
			delete(m, key)
		}
	case *string:
		if value != nil {
			m[key] = *value
		} else {
			delete(m, key)
		}
	case common.MapStr:
		if len(value) > 0 {
			newValMap := common.MapStr{}
			for k, v := range value {
				Add(newValMap, k, v)
			}
			if len(newValMap) > 0 {
				m[key] = newValMap
			} else {
				delete(m, key)
			}
		} else {
			delete(m, key)
		}
	case map[string]interface{}:
		if len(value) > 0 {
			newValMap := map[string]interface{}{}
			for k, v := range value {
				Add(newValMap, k, v)
			}
			if len(newValMap) > 0 {
				m[key] = newValMap
			} else {
				delete(m, key)
			}
		} else {
			delete(m, key)
		}
	case json.Number:
		if floatVal, err := value.Float64(); err != nil {
			Add(m, key, value.String())
		} else {
			Add(m, key, floatVal)
		}
	case float64:
		if value == float64(int64(value)) {
			m[key] = int64(value)
		} else {
			m[key] = common.Float(value)
		}
	case *float64:
		if value != nil {
			m[key] = *value
		} else {
			delete(m, key)
		}
	case float32:
		if value == float32(int32(value)) {
			m[key] = int32(value)
		} else {
			m[key] = common.Float(value)
		}
	case string, bool, complex64, complex128:
		m[key] = val
	case int, int8, int16, int32, int64, uint, uint8, uint32, uint64:
		m[key] = val
	default:
		v := reflect.ValueOf(val)
		switch v.Type().Kind() {
		case reflect.Slice, reflect.Array:
			if v.Len() == 0 {
				delete(m, key)
			} else {
				m[key] = val
			}

		// do not store values of following type
		// has been rejected so far by the libbeat normalization
		case reflect.Interface, reflect.Chan, reflect.Func, reflect.UnsafePointer, reflect.Uintptr:

		default:
			m[key] = val
		}
	}
}

// MergeAdd modifies `m` *in place*, inserting `valu` at the given `key`.
// If `key` doesn't exist in m(at the top level), it gets created.
// If the value under `key` is not a map, MergeAdd does nothing.
func MergeAdd(m common.MapStr, key string, val common.MapStr) {
	if m == nil || key == "" || val == nil || len(val) == 0 {
		return
	}

	if _, ok := m[key]; !ok {
		m[key] = common.MapStr{}
	}

	if nested, ok := m[key].(common.MapStr); ok {
		for k, v := range val {
			Add(nested, k, v)
		}
	} else if nested, ok := m[key].(map[string]interface{}); ok {
		for k, v := range val {
			Add(nested, k, v)
		}
	}
}

func MillisAsMicros(ms float64) common.MapStr {
	m := common.MapStr{}
	m["us"] = int(ms * 1000)
	return m
}

func TimeAsMicros(t time.Time) common.MapStr {
	if t.IsZero() {
		return nil
	}

	m := common.MapStr{}
	m["us"] = t.UnixNano() / 1000
	return m
}

func Prune(m common.MapStr) common.MapStr {
	for k, v := range m {
		if v == nil {
			delete(m, k)
		}
	}
	return m
}

func AddId(fields common.MapStr, key string, id *string) {
	if id != nil && *id != "" {
		fields[key] = common.MapStr{"id": *id}
	}
}
