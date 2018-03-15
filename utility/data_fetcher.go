package utility

import (
	"errors"
	"time"
)

var (
	typeErr = errors.New("Invalid type for field.")
	nilErr  = errors.New("Mandatory field missing.")
)

type DataFetcher struct {
	Err error
}

func (d *DataFetcher) Float64(base map[string]interface{}, key string, keys ...string) float64 {
	val := d.getDeep(base, keys...)[key]
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
	val := d.getDeep(base, keys...)[key]
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
	}
	d.Err = typeErr
	return nil
}

func (d *DataFetcher) Interface(base map[string]interface{}, key string, keys ...string) interface{} {
	return d.getDeep(base, keys...)[key]
}

func (d *DataFetcher) Int(base map[string]interface{}, key string, keys ...string) int {
	if val := d.IntPtr(base, key, keys...); val != nil {
		return *val
	}
	val := d.getDeep(base, keys...)[key]
	if val == nil {
		d.Err = nilErr
	}
	return 0
}

func (d *DataFetcher) StringPtr(base map[string]interface{}, key string, keys ...string) *string {
	val := d.getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valStr, ok := val.(string); ok {
		return &valStr
	}
	d.Err = typeErr
	return nil
}

func (d *DataFetcher) String(base map[string]interface{}, key string, keys ...string) string {
	if val := d.StringPtr(base, key, keys...); val != nil {
		return *val
	}
	val := d.getDeep(base, keys...)[key]
	if val == nil {
		d.Err = nilErr
	}
	return ""
}

func (d *DataFetcher) StringArr(base map[string]interface{}, key string, keys ...string) []string {
	val := d.getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valArr, ok := val.([]string); ok {
		return valArr
	}
	if valArr, ok := d.getDeep(base, keys...)[key].([]interface{}); ok {
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

func (d *DataFetcher) InterfaceArr(base map[string]interface{}, key string, keys ...string) []interface{} {
	val := d.getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valArr, ok := val.([]interface{}); ok {
		return valArr
	}
	d.Err = typeErr
	return nil
}

func (d *DataFetcher) BoolPtr(base map[string]interface{}, key string, keys ...string) *bool {
	val := d.getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valPtr, ok := val.(bool); ok {
		return &valPtr
	}
	d.Err = typeErr
	return nil
}

func (d *DataFetcher) MapStr(base map[string]interface{}, key string, keys ...string) map[string]interface{} {
	val := d.getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valMapStr, ok := val.(map[string]interface{}); ok {
		return valMapStr
	}
	d.Err = typeErr
	return nil
}

func (d *DataFetcher) TimeRFC3339(base map[string]interface{}, key string, keys ...string) time.Time {
	val := d.getDeep(base, keys...)[key]
	if valStr, ok := val.(string); ok {
		valTime, Err := time.Parse(time.RFC3339, valStr)
		if Err == nil {
			return valTime
		}
	} else if val == nil {
		d.Err = nilErr
		return time.Time{}
	}
	d.Err = typeErr
	return time.Time{}
}

func (d *DataFetcher) getDeep(raw map[string]interface{}, keys ...string) map[string]interface{} {
	if raw == nil {
		return nil
	}
	if len(keys) == 0 {
		return raw
	}
	raw, _ = raw[keys[0]].(map[string]interface{})
	return d.getDeep(raw, keys[1:]...)
}
