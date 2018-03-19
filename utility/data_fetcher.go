package utility

import (
	"errors"
	"time"

	"github.com/elastic/beats/libbeat/common"
)

type DataFetcher struct {
	Err error
}

var (
	typeErr = errors.New("Invalid type for field")
	nilErr  = errors.New("Mandatory field missing")
)

func (d *DataFetcher) Float64(base map[string]interface{}, key string, keys ...string) float64 {
	val := getDeep(base, keys...)[key]
	if valFloat, ok := val.(float64); ok {
		return valFloat
	} else if val == nil {
		d.Err = nilErr
	} else {
		d.Err = typeErr
	}
	return 0.0
}

func (d *DataFetcher) IntPtr(base map[string]interface{}, key string, keys ...string) *int {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valFloat, ok := val.(float64); ok {
		valInt := int(valFloat)
		if valFloat == float64(valInt) {
			return &valInt
		}
	} else if valFloat, ok := val.(float32); ok {
		valInt := int(valFloat)
		if valFloat == float32(valInt) {
			return &valInt
		}
	} else if valInt, ok := val.(int); ok {
		return &valInt
	} else if valIntPtr, ok := val.(*int); ok {
		return valIntPtr
	}
	d.Err = typeErr
	return nil
}

func (d *DataFetcher) Int(base map[string]interface{}, key string, keys ...string) int {
	if val := d.IntPtr(base, key, keys...); val != nil {
		return *val
	}
	if d.Err == nil {
		d.Err = nilErr
	}
	return 0
}

func (d *DataFetcher) StringPtr(base map[string]interface{}, key string, keys ...string) *string {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valStr, ok := val.(string); ok {
		return &valStr
	} else if valStrPtr, ok := val.(*string); ok {
		return valStrPtr
	}
	d.Err = typeErr
	return nil
}

func (d *DataFetcher) String(base map[string]interface{}, key string, keys ...string) string {
	if val := d.StringPtr(base, key, keys...); val != nil {
		return *val
	}
	val := getDeep(base, keys...)[key]
	if val == nil {
		d.Err = nilErr
	}
	return ""
}

func (d *DataFetcher) StringArr(base map[string]interface{}, key string, keys ...string) []string {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valArr, ok := val.([]string); ok {
		return valArr
	}
	if valArr, ok := getDeep(base, keys...)[key].([]interface{}); ok {
		strArr := make([]string, len(valArr))
		for idx, v := range valArr {
			if valStr, ok := v.(string); ok {
				strArr[idx] = valStr
			} else {
				d.Err = typeErr
				return nil
			}
		}
		return strArr
	}
	d.Err = typeErr
	return nil
}

func (d *DataFetcher) Interface(base map[string]interface{}, key string, keys ...string) interface{} {
	return getDeep(base, keys...)[key]
}

func (d *DataFetcher) InterfaceArr(base map[string]interface{}, key string, keys ...string) []interface{} {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valArr, ok := val.([]interface{}); ok {
		return valArr
	}
	d.Err = typeErr
	return nil
}

func (d *DataFetcher) BoolPtr(base map[string]interface{}, key string, keys ...string) *bool {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valPtr, ok := val.(*bool); ok {
		return valPtr
	} else if valBool, ok := val.(bool); ok {
		return &valBool
	}
	d.Err = typeErr
	return nil
}

func (d *DataFetcher) MapStr(base map[string]interface{}, key string, keys ...string) map[string]interface{} {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valMapStr, ok := val.(map[string]interface{}); ok {
		return valMapStr
	} else if valMapStr, ok := val.(common.MapStr); ok {
		return valMapStr
	}
	d.Err = typeErr
	return nil
}

func (d *DataFetcher) TimeRFC3339(base map[string]interface{}, key string, keys ...string) time.Time {
	val := getDeep(base, keys...)[key]
	if valStr, ok := val.(string); ok {
		if valTime, err := time.Parse(time.RFC3339, valStr); err == nil {
			return valTime
		}
	} else if valTime, ok := val.(time.Time); ok {
		return valTime
	} else if val == nil {
		d.Err = nilErr
		return time.Time{}
	}
	d.Err = typeErr
	return time.Time{}
}

func getDeep(raw map[string]interface{}, keys ...string) map[string]interface{} {
	if raw == nil {
		return nil
	}
	if len(keys) == 0 {
		return raw
	}
	if valMap, ok := raw[keys[0]].(map[string]interface{}); ok {
		return getDeep(valMap, keys[1:]...)
	} else if valMap, ok := raw[keys[0]].(common.MapStr); ok {
		return getDeep(valMap, keys[1:]...)
	}
	return nil
}
