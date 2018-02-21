package utility

import (
	"github.com/elastic/beats/libbeat/common"
)

func Add(m common.MapStr, key string, val interface{}) {
	if m == nil || key == "" {
		return
	}
	switch val.(type) {
	case *bool:
		if newVal := val.(*bool); newVal != nil {
			m[key] = *newVal
		}
	case *int:
		if newVal := val.(*int); newVal != nil {
			m[key] = *newVal
		}
	case *string:
		if newVal := val.(*string); newVal != nil {
			m[key] = *newVal
		}
	case common.MapStr:
		if valMap := val.(common.MapStr); len(valMap) > 0 {
			for k, v := range valMap {
				Add(valMap, k, v)
			}
			m[key] = valMap
			if len(valMap) > 0 {
				m[key] = valMap
			}
		}
	case map[string]interface{}:
		valMap := val.(map[string]interface{})
		if len(valMap) > 0 {
			for k, v := range valMap {
				Add(valMap, k, v)
			}
			if len(valMap) > 0 {
				m[key] = valMap
			}
		}
	case []string:
		if valArr := val.([]string); len(valArr) > 0 {
			m[key] = valArr
		}
	case []common.MapStr:
		if valArr := val.([]common.MapStr); len(valArr) > 0 {
			m[key] = valArr
		}
	default:
		if val != nil {
			m[key] = val

		}
	}
}

// MergeAdd modifies `data` *in place*, inserting `values` at the given `key`.
// If `key` doesn't exist in data (at the top level), it gets created.
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

func AddStrWithDefault(m common.MapStr, key string, val *string, defaultVal string) {
	if m == nil || key == "" {
		return
	}
	if val != nil {
		m[key] = *val
	} else if defaultVal != "" {
		m[key] = defaultVal
	}
}
