package utility

import (
	"github.com/elastic/beats/libbeat/common"
)

func AddStrWithDefault(m common.MapStr, key string, val *string, defaultVal string) {
	if val != nil {
		m[key] = *val
	} else if defaultVal != "" {
		m[key] = defaultVal
	}
}

func Add(m common.MapStr, key string, val interface{}) {

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
			m[key] = valMap
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

func MillisAsMicros(ms float64) common.MapStr {
	m := common.MapStr{}
	m["us"] = int(ms * 1000)
	return m
}
