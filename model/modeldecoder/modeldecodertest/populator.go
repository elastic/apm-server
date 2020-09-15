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
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder/nullable"
)

// InitStructValues iterates through the struct fields represented by
// the given reflect.Value and initializes all fields with
// some arbitrary value.
func InitStructValues(i interface{}) {
	SetStructValues(i, "unknown", 1, true)
}

// SetStructValues iterates through the struct fields represented by
// the given reflect.Value and initializes all fields with
// the given values for strings and integers.
func SetStructValues(in interface{}, vStr string, vInt int, vBool bool) {
	IterateStruct(in, func(f reflect.Value, key string) {
		var newVal interface{}
		switch v := f.Interface().(type) {
		case map[string]interface{}:
			newVal = map[string]interface{}{vStr: vStr}
		case common.MapStr:
			newVal = common.MapStr{vStr: vStr}
		case map[string]map[string]float64:
			newVal = map[string]map[string]float64{
				vStr: map[string]float64{vStr: float64(vInt) + 0.5}}
		case []string:
			newVal = []string{vStr}
		case []int:
			newVal = []int{vInt, vInt}
		case nullable.String:
			v.Set(vStr)
			newVal = v
		case nullable.Int:
			v.Set(vInt)
			newVal = v
		case nullable.Interface:
			if strings.Contains(key, "port") {
				v.Set(vInt)
			} else {
				v.Set(vStr)
			}
			newVal = v
		case nullable.Bool:
			v.Set(vBool)
			newVal = v
		case nullable.Float64:
			v.Set(float64(vInt) + 0.5)
			newVal = v
		case nullable.TimeMicrosUnix:
			v.Set(time.Now())
			newVal = v
		case nullable.HTTPHeader:
			v.Set(http.Header{vStr: []string{vStr, vStr}})
			newVal = v
		default:
			// IterateStruct recursively iterates over struct fields,
			// therefore we can return for the struct itself.
			if f.Type().Kind() == reflect.Struct {
				return
			}
			panic(fmt.Sprintf("unhandled type %T for key %s", f.Type().Kind(), key))
		}
		f.Set(reflect.ValueOf(newVal))
	})
}

// SetZeroStructValues iterates through the struct fields represented by
// the given reflect.Value and sets all fields to their zero values.
func SetZeroStructValues(i interface{}) {
	IterateStruct(i, func(f reflect.Value, key string) {
		f.Set(reflect.Zero(f.Type()))
	})
}

// SetZeroStructValue iterates through the struct fields represented by
// the given reflect.Value, sets a field to its zero value,
// calls the callback function and resets the field to its original value
func SetZeroStructValue(i interface{}, callback func(string)) {
	IterateStruct(i, func(f reflect.Value, key string) {
		original := reflect.ValueOf(f.Interface())
		defer f.Set(original) // reset original value
		f.Set(reflect.Zero(f.Type()))
		callback(key)
	})
}

// AssertStructValues recursively walks through the given struct and asserts
// that values are equal to expected values
func AssertStructValues(t *testing.T, i interface{}, isException func(string) bool,
	vStr string, vInt int, vBool bool, vIP net.IP) {
	IterateStruct(i, func(f reflect.Value, key string) {
		if isException(key) {
			return
		}
		fVal := f.Interface()
		var newVal interface{}
		switch fVal.(type) {
		case map[string]interface{}:
			newVal = map[string]interface{}{vStr: vStr}
		case common.MapStr:
			newVal = common.MapStr{vStr: vStr}
		case *model.Labels:
			newVal = &model.Labels{vStr: vStr}
		case *model.Custom:
			newVal = &model.Custom{vStr: vStr}
		case model.TransactionMarks:
			newVal = model.TransactionMarks{vStr: model.TransactionMark{vStr: float64(vInt) + 0.5}}
		case []string:
			newVal = []string{vStr}
		case []int:
			newVal = []int{vInt, vInt}
		case string:
			newVal = vStr
		case *string:
			newVal = &vStr
		case int:
			newVal = vInt
		case *int:
			newVal = &vInt
		case float64:
			newVal = float64(vInt) + 0.5
		case *float64:
			val := float64(vInt) + 0.5
			newVal = &val
		case net.IP:
			newVal = vIP
		case bool:
			newVal = vBool
		case *bool:
			newVal = &vBool
		case http.Header:
			newVal = http.Header{vStr: []string{vStr, vStr}}
		default:
			// the populator recursively iterates over struct and structPtr
			// calling this function for all fields;
			// it is enough to only assert they are not zero here
			if f.Type().Kind() == reflect.Struct {
				assert.NotZero(t, f, key)
				return
			}
			if f.Type().Kind() == reflect.Ptr && f.Type().Elem().Kind() == reflect.Struct {
				assert.NotZero(t, f, key)
				return
			}
			panic(fmt.Sprintf("unhandled type %T for key %s", f.Type().Kind(), key))
		}
		assert.Equal(t, newVal, fVal, key)
	})
}

// IterateStruct iterates through the struct fields represented by
// the given reflect.Value and calls the given function on every field.
func IterateStruct(i interface{}, fn func(reflect.Value, string)) {
	val := reflect.ValueOf(i)
	if val.Kind() != reflect.Ptr {
		panic("expected pointer to struct as parameter")
	}
	iterateStruct(val.Elem(), "", fn)
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

		switch fTyp.Kind() {
		case reflect.Ptr:
			if !f.IsZero() && fTyp.Elem().Kind() == reflect.Struct {
				iterateStruct(f.Elem(), fKey, fn)
			}
		case reflect.Struct:
			switch f.Interface().(type) {
			case nullable.String, nullable.Int, nullable.Bool, nullable.Float64,
				nullable.Interface, nullable.HTTPHeader, nullable.TimeMicrosUnix:
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
