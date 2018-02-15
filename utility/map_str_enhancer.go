package utility

import (
	"github.com/elastic/beats/libbeat/common"
)

func AddStrPtr(m common.MapStr, key string, val *string) {
	if val != nil {
		m[key] = val
	}
}

func AddStrPtrWithDefault(m common.MapStr, key string, val *string, defaultVal *string) {
	if val != nil {
		m[key] = val
	} else if defaultVal != nil {
		m[key] = defaultVal
	}
}

func AddStrArray(m common.MapStr, key string, val []string) {
	if len(val) > 0 {
		m[key] = val
	}
}

func AddIntPtr(m common.MapStr, key string, val *int) {
	if val != nil {
		m[key] = val
	}
}

func AddBoolPtr(m common.MapStr, key string, val *bool) {
	if val != nil {
		m[key] = val
	}
}

func AddInterface(m common.MapStr, key string, val interface{}) {
	if val != nil {
		m[key] = val
	}
}

func AddCommonMapStr(m common.MapStr, key string, val common.MapStr) {
	if len(val) > 0 {
		m[key] = val
	}
}

func AddCommonMapStrArray(m common.MapStr, key string, val []common.MapStr) {
	if len(val) > 0 {
		m[key] = val
	}
}

func AddMillis(m common.MapStr, key string, val *float64) {
	if val == nil {
		return
	}
	ms := int(*val * 1000)
	m[key] = common.MapStr{"us": &ms}
}
