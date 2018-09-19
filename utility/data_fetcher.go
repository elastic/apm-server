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
	"errors"
	"time"

	"github.com/elastic/beats/libbeat/common"
)

type ManualDecoder struct {
	Err error
}

var (
	fetchErr = errors.New("Error fetching field")
)

func (d *ManualDecoder) Float64(base map[string]interface{}, key string, keys ...string) float64 {
	val := getDeep(base, keys...)[key]
	if valFloat, ok := val.(float64); ok {
		return valFloat
	} else if valNumber, ok := val.(json.Number); ok {
		if valFloat, err := valNumber.Float64(); err != nil {
			d.Err = err
		} else {
			return valFloat
		}
	}

	d.Err = fetchErr
	return 0.0
}

func (d *ManualDecoder) Float64Ptr(base map[string]interface{}, key string, keys ...string) *float64 {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valFloat, ok := val.(float64); ok {
		return &valFloat
	} else if valNumber, ok := val.(json.Number); ok {
		if valFloat, err := valNumber.Float64(); err != nil {
			d.Err = err
		} else {
			return &valFloat
		}
	}

	d.Err = fetchErr
	return nil
}

func (d *ManualDecoder) IntPtr(base map[string]interface{}, key string, keys ...string) *int {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valNumber, ok := val.(json.Number); ok {
		if valInt, err := valNumber.Int64(); err != nil {
			d.Err = err
		} else {
			i := int(valInt)
			return &i
		}
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
	}
	d.Err = fetchErr
	return nil
}

func (d *ManualDecoder) Int64Ptr(base map[string]interface{}, key string, keys ...string) *int64 {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valNumber, ok := val.(json.Number); ok {
		if valInt, err := valNumber.Int64(); err != nil {
			d.Err = err
		} else {
			i := int64(valInt)
			return &i
		}
	} else if valFloat, ok := val.(float64); ok {
		valInt := int64(valFloat)
		if valFloat == float64(valInt) {
			return &valInt
		}
	}
	d.Err = fetchErr
	return nil
}

func (d *ManualDecoder) Int(base map[string]interface{}, key string, keys ...string) int {
	if val := d.IntPtr(base, key, keys...); val != nil {
		return *val
	}
	d.Err = fetchErr
	return 0
}

func (d *ManualDecoder) StringPtr(base map[string]interface{}, key string, keys ...string) *string {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valStr, ok := val.(string); ok {
		return &valStr
	}
	d.Err = fetchErr
	return nil
}

func (d *ManualDecoder) String(base map[string]interface{}, key string, keys ...string) string {
	if val := d.StringPtr(base, key, keys...); val != nil {
		return *val
	}
	d.Err = fetchErr
	return ""
}

func (d *ManualDecoder) StringArr(base map[string]interface{}, key string, keys ...string) []string {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	}
	if valArr, ok := getDeep(base, keys...)[key].([]interface{}); ok {
		strArr := make([]string, len(valArr))
		for idx, v := range valArr {
			if valStr, ok := v.(string); ok {
				strArr[idx] = valStr
			} else {
				d.Err = fetchErr
				return nil
			}
		}
		return strArr
	}
	d.Err = fetchErr
	return nil
}

func (d *ManualDecoder) Interface(base map[string]interface{}, key string, keys ...string) interface{} {
	return getDeep(base, keys...)[key]
}

func (d *ManualDecoder) InterfaceArr(base map[string]interface{}, key string, keys ...string) []interface{} {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valArr, ok := val.([]interface{}); ok {
		return valArr
	}
	d.Err = fetchErr
	return nil
}

func (d *ManualDecoder) BoolPtr(base map[string]interface{}, key string, keys ...string) *bool {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valBool, ok := val.(bool); ok {
		return &valBool
	}
	d.Err = fetchErr
	return nil
}

func (d *ManualDecoder) MapStr(base map[string]interface{}, key string, keys ...string) map[string]interface{} {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valMapStr, ok := val.(map[string]interface{}); ok {
		return valMapStr
	}
	d.Err = fetchErr
	return nil
}

func (d *ManualDecoder) TimeRFC3339(base map[string]interface{}, key string, keys ...string) time.Time {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return time.Time{}
	}
	if valStr, ok := val.(string); ok {
		if valTime, err := time.Parse(time.RFC3339, valStr); err == nil {
			return valTime
		}
	}
	d.Err = fetchErr
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
