package utility

import (
	"encoding/json"
	"reflect"

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
		if newVal := val.(*bool); newVal != nil {
			m[key] = *newVal
		} else {
			delete(m, key)
		}
	case *int:
		if newVal := val.(*int); newVal != nil {
			m[key] = *newVal
		} else {
			delete(m, key)
		}
	case *string:
		if newVal := val.(*string); newVal != nil {
			m[key] = *newVal
		} else {
			delete(m, key)
		}
	case common.MapStr:
		if valMap := val.(common.MapStr); len(valMap) > 0 {
			for k, v := range valMap {
				Add(valMap, k, v)
			}
			if len(valMap) > 0 {
				m[key] = valMap
			} else {
				delete(m, key)
			}
		} else {
			delete(m, key)
		}
	case map[string]interface{}:
		if valMap := val.(map[string]interface{}); len(valMap) > 0 {
			for k, v := range valMap {
				Add(valMap, k, v)
			}
			if len(valMap) > 0 {
				m[key] = valMap
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
		floatVal := val.(float64)
		if floatVal == float64(int64(floatVal)) {
			m[key] = int64(floatVal)
		} else {
			m[key] = common.Float(floatVal)
		}
	case float32:
		floatVal := val.(float32)
		if floatVal == float32(int32(floatVal)) {
			m[key] = int32(floatVal)
		} else {
			m[key] = common.Float(floatVal)
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
