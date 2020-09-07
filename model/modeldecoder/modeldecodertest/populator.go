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

package modeldecodertest

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/elastic/apm-server/model/modeldecoder/nullable"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/stretchr/testify/assert"
)

func InitStructValues(val reflect.Value) {
	SetStructValues(val, "initialized", 1)
}

func SetStructValues(val reflect.Value, s string, i int) {
	iterateStruct(val.Elem(), "", func(f reflect.Value, key string) {
		var newVal interface{}
		switch v := f.Interface().(type) {
		case map[string]interface{}:
			newVal = map[string]interface{}{s: s}
		case common.MapStr:
			newVal = common.MapStr{s: s}
		case []string:
			newVal = []string{s}
		case []int:
			newVal = []int{i, i}
		case nullable.String:
			v.Set(s)
			newVal = v
		case nullable.Int:
			v.Set(i)
			newVal = v
		case nullable.Interface:
			v.Set(s)
			newVal = v
		default:
			if f.Type().Kind() == reflect.Struct {
				return
			}
			panic(fmt.Sprintf("unhandled type %T for key %s", f.Type().Kind(), key))
		}
		f.Set(reflect.ValueOf(newVal))
	})
}

func SetZeroStructValues(val reflect.Value) {
	iterateStruct(val.Elem(), "", func(f reflect.Value, key string) {
		f.Set(reflect.Zero(f.Type()))
	})
}

func SetZeroStructValue(val reflect.Value, callback func(string)) {
	iterateStruct(val.Elem(), "", func(f reflect.Value, key string) {
		original := reflect.ValueOf(f.Interface())
		defer f.Set(original) // reset original value
		f.Set(reflect.Zero(f.Type()))
		callback(key)
	})
}

func AssertStructValues(t *testing.T, val reflect.Value, s string, i int) {
	iterateStruct(val.Elem(), "", func(f reflect.Value, key string) {
		fVal := f.Interface()
		var newVal interface{}
		switch fVal.(type) {
		case map[string]interface{}:
			newVal = map[string]interface{}{s: s}
		case common.MapStr:
			newVal = common.MapStr{s: s}
		case []string:
			newVal = []string{s}
		case []int:
			newVal = []int{i, i}
		case string:
			newVal = s
		case int:
			newVal = i
		case *int:
			iptr := f.Interface().(*int)
			fVal = *iptr
			newVal = i
		case net.IP:
		default:
			if f.Type().Kind() == reflect.Struct {
				return
			}
			panic(fmt.Sprintf("unhandled type %T for key %s", f.Type().Kind(), key))
		}
		if strings.HasPrefix(key, "UserAgent") || key == "Client.IP" || key == "System.IP" {
			// these values are not set by modeldecoder
			return
		}
		assert.Equal(t, newVal, fVal, key)
	})
}

func iterateStruct(v reflect.Value, key string, fn func(f reflect.Value, fKey string)) {
	t := v.Type()
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("iterateStruct: invalid typ %T", t.Kind()))
	}
	if key != "" {
		key += "."
	}
	var fKey string
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.CanSet() {
			continue
		}
		stf := t.Field(i)
		fTyp := stf.Type
		name := jsonName(stf)
		if name == "" {
			name = stf.Name
		}
		fKey = fmt.Sprintf("%s%s", key, name)

		if fTyp.Kind() == reflect.Struct {
			switch f.Interface().(type) {
			case nullable.String, nullable.Int, nullable.Interface:
			default:
				iterateStruct(f, fKey, fn)
			}
		}
		fn(f, fKey)
	}
}

func jsonName(f reflect.StructField) string {
	tag, ok := f.Tag.Lookup("json")
	if !ok || tag == "-" {
		return ""
	}
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
