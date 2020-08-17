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
	"net"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/elastic/beats/v7/libbeat/common"
)

type ManualDecoder struct {
	Err error
}

func ErrFetch(field string, path []string) error {
	if path != nil {
		field = strings.Join(path, ".") + "." + field
	}
	return errors.New("error fetching field " + field)
}

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

	d.Err = ErrFetch(key, keys)
	return 0.0
}

func (d *ManualDecoder) Float64WithDefault(base map[string]interface{}, def float64, key string, keys ...string) float64 {
	if f := d.Float64Ptr(base, key, keys...); f != nil {
		return *f
	}
	return def
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

	d.Err = ErrFetch(key, keys)
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
	d.Err = ErrFetch(key, keys)
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
	d.Err = ErrFetch(key, keys)
	return nil
}

func (d *ManualDecoder) Int(base map[string]interface{}, key string, keys ...string) int {
	if val := d.IntPtr(base, key, keys...); val != nil {
		return *val
	}
	d.Err = ErrFetch(key, keys)
	return 0
}

func (d *ManualDecoder) StringPtr(base map[string]interface{}, key string, keys ...string) *string {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	}
	if valStr, ok := val.(string); ok {
		return &valStr
	}
	d.Err = ErrFetch(key, keys)
	return nil
}

func (d *ManualDecoder) String(base map[string]interface{}, key string, keys ...string) string {
	if val := d.StringPtr(base, key, keys...); val != nil {
		return *val
	}
	d.Err = ErrFetch(key, keys)
	return ""
}

// NetIP extracts the value for key nested under keys from base and tries to decode as net.IP.
// On error the decoder.Err is updated.
func (d *ManualDecoder) NetIP(base map[string]interface{}, key string, keys ...string) net.IP {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	}
	if valStr, ok := val.(string); ok {
		return ParseIP(valStr)
	}
	d.Err = ErrFetch(key, keys)
	return nil
}

func (d *ManualDecoder) StringArr(base map[string]interface{}, key string, keys ...string) []string {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	}
	arr := getDeep(base, keys...)[key]
	if valArr, ok := arr.([]interface{}); ok {
		strArr := make([]string, len(valArr))
		for idx, v := range valArr {
			if valStr, ok := v.(string); ok {
				strArr[idx] = valStr
			} else {
				d.Err = ErrFetch(key, keys)
				return nil
			}
		}
		return strArr
	}
	if strArr, ok := arr.([]string); ok {
		return strArr
	}
	d.Err = ErrFetch(key, keys)
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
	d.Err = ErrFetch(key, keys)
	return nil
}

func (d *ManualDecoder) BoolPtr(base map[string]interface{}, key string, keys ...string) *bool {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valBool, ok := val.(bool); ok {
		return &valBool
	}
	d.Err = ErrFetch(key, keys)
	return nil
}

func (d *ManualDecoder) MapStr(base map[string]interface{}, key string, keys ...string) map[string]interface{} {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return nil
	} else if valMapStr, ok := val.(map[string]interface{}); ok {
		return valMapStr
	}
	d.Err = ErrFetch(key, keys)
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
	d.Err = ErrFetch(key, keys)
	return time.Time{}
}

func (d *ManualDecoder) TimeEpochMicro(base map[string]interface{}, key string, keys ...string) time.Time {
	val := getDeep(base, keys...)[key]
	if val == nil {
		return time.Time{}
	}

	if valNum, ok := val.(json.Number); ok {
		if t, err := valNum.Int64(); err == nil {
			sec := t / 1000000
			microsec := t - (sec * 1000000)
			return time.Unix(sec, microsec*1000).UTC()
		}
	}
	d.Err = ErrFetch(key, keys)
	return time.Time{}
}

func (d *ManualDecoder) Headers(base map[string]interface{}, fieldName string) http.Header {

	h := d.MapStr(base, fieldName)
	if d.Err != nil || len(h) == 0 {
		return nil
	}
	httpHeader := http.Header{}
	for key, val := range h {
		if v, ok := val.(string); ok {
			httpHeader.Add(key, v)
			continue
		}
		vals := d.StringArr(h, key)
		if d.Err != nil {
			return nil
		}
		for _, v := range vals {
			httpHeader.Add(key, v)
		}
	}
	return httpHeader
}

// UserAgentHeader fetches all `user-agent` values from a given header and combines them into one string.
// Values are separated by `;`.
func (d *ManualDecoder) UserAgentHeader(header http.Header) string {
	return strings.Join(header[textproto.CanonicalMIMEHeaderKey("User-Agent")], ", ")
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
