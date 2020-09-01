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
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/elastic/beats/v7/libbeat/common"
)

// Set takes a map and changes key to point to the provided value.
// In case the provided value is nil or of length 0, the key is deleted from the map .
func Set(m common.MapStr, key string, val interface{}) {
	update(m, key, val, true)
}

func update(m common.MapStr, key string, val interface{}, remove bool) {
	if m == nil || key == "" {
		return
	}

	if val == nil {
		if remove {
			delete(m, key)
		}
		return
	}

	switch value := val.(type) {
	case *bool:
		if value != nil {
			m[key] = *value
		} else if remove {
			delete(m, key)
		}
	case *int:
		if value != nil {
			m[key] = *value
		} else if remove {
			delete(m, key)
		}
	case *int64:
		if newVal := val.(*int64); newVal != nil {
			m[key] = *newVal
		} else if remove {
			delete(m, key)
		}
	case *string:
		if value != nil {
			m[key] = *value
		} else if remove {
			delete(m, key)
		}
	case common.MapStr:
		if len(value) > 0 {
			newValMap := common.MapStr{}
			for k, v := range value {
				update(newValMap, k, v, remove)
			}
			if len(newValMap) > 0 {
				m[key] = newValMap
			} else if remove {
				delete(m, key)
			}
		} else if remove {
			delete(m, key)
		}
	case map[string]interface{}:
		if len(value) > 0 {
			newValMap := map[string]interface{}{}
			for k, v := range value {
				update(newValMap, k, v, remove)
			}
			if len(newValMap) > 0 {
				m[key] = newValMap
			} else if remove {
				delete(m, key)
			}
		} else if remove {
			delete(m, key)
		}
	case json.Number:
		if floatVal, err := value.Float64(); err != nil {
			update(m, key, value.String(), remove)
		} else {
			update(m, key, floatVal, remove)
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
		} else if remove {
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
	case http.Header:
		if value != nil {
			m[key] = value
		} else if remove {
			delete(m, key)
		}
	default:
		v := reflect.ValueOf(val)
		switch v.Type().Kind() {
		case reflect.Slice, reflect.Array:
			if v.Len() > 0 {
				m[key] = val
			} else if remove {
				delete(m, key)
			}

		// do not store values of following type
		// has been rejected so far by the libbeat normalization
		case reflect.Interface, reflect.Chan, reflect.Func, reflect.UnsafePointer, reflect.Uintptr:

		default:
			m[key] = val
		}
	}
}

// DeepUpdate splits the key by '.' and merges the given value at m[de-dottedKeys].
func DeepUpdate(m common.MapStr, dottedKeys string, val interface{}) {
	if m == nil {
		m = common.MapStr{}
	}
	keys := strings.Split(dottedKeys, ".")
	if len(keys) == 0 {
		return
	}
	reverse(keys)
	v := val
	for _, k := range keys {
		subMap := common.MapStr{}
		update(subMap, k, v, false)
		v = subMap
	}
	m.DeepUpdate(v.(common.MapStr))
}

func reverse(slice []string) {
	size := len(slice)
	for i := 0; i < len(slice)/2; i++ {
		slice[i], slice[size-i-1] = slice[size-i-1], slice[i]
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

func AddID(fields common.MapStr, key, id string) {
	if id != "" {
		fields[key] = common.MapStr{"id": id}
	}
}
