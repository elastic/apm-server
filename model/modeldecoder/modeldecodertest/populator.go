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

	"github.com/elastic/apm-server/model/modeldecoder/nullable"
)

// Values used for populating the model structs
type Values struct {
	Str        string
	Int        int
	Float      float64
	Bool       bool
	Time       time.Time
	IP         net.IP
	HTTPHeader http.Header
	// N controls how many elements are added to a slice or a map
	N int
}

// DefaultValues returns a Values struct initialized with non-zero values
func DefaultValues() *Values {
	initTime, _ := time.Parse(time.RFC3339, "2020-10-10T10:00:00Z")
	return &Values{
		Str:        "init",
		Int:        1,
		Float:      0.5,
		Bool:       true,
		Time:       initTime,
		IP:         net.ParseIP("127.0.0.1"),
		HTTPHeader: http.Header{http.CanonicalHeaderKey("user-agent"): []string{"a", "b", "c"}},
		N:          3,
	}
}

// NonDefaultValues returns a Values struct initialized with non-zero values
func NonDefaultValues() *Values {
	updatedTime, _ := time.Parse(time.RFC3339, "2020-12-10T10:00:00Z")
	return &Values{
		Str:        "overwritten",
		Int:        12,
		Float:      3.5,
		Bool:       false,
		Time:       updatedTime,
		IP:         net.ParseIP("192.168.0.1"),
		HTTPHeader: http.Header{http.CanonicalHeaderKey("user-agent"): []string{"d", "e"}},
		N:          2,
	}
}

// Update arbitrary values
func (v *Values) Update(args ...interface{}) {
	for _, arg := range args {
		switch a := arg.(type) {
		case string:
			v.Str = a
		case int:
			v.Int = a
		case float64:
			v.Float = a
		case bool:
			v.Bool = a
		case time.Time:
			v.Time = a
		case net.IP:
			v.IP = a
		case http.Header:
			v.HTTPHeader = a
		default:
			panic(fmt.Sprintf("Values Merge: value type for %v not implemented", a))
		}
	}
}

// InitStructValues iterates through the struct fields represented by
// the given reflect.Value and initializes all fields with
// some arbitrary value.
func InitStructValues(i interface{}) {
	SetStructValues(i, DefaultValues())
}

// SetStructValuesOption is the type of an option which may be passed into
// SetStructValues to override the value to which a field is set. If the
// option returns false, then the field will not be updated and no more options
// will be invoked.
type SetStructValuesOption func(key string, field, value reflect.Value) bool

// SetStructValues iterates through the struct fields represented by
// the given reflect.Value and initializes all fields with the provided values
func SetStructValues(in interface{}, values *Values, opts ...SetStructValuesOption) {
	IterateStruct(in, func(f reflect.Value, key string) {
		fieldVal := f
		switch fKind := f.Kind(); fKind {
		case reflect.String:
			fieldVal = reflect.ValueOf(values.Str)
		case reflect.Int:
			fieldVal = reflect.ValueOf(values.Int)
		case reflect.Slice:
			var elemVal reflect.Value
			switch v := f.Interface().(type) {
			case []string:
				elemVal = reflect.ValueOf(values.Str)
			case []int:
				elemVal = reflect.ValueOf(values.Int)
			case net.IP:
				fieldVal = reflect.ValueOf(values.IP)
			default:
				if f.Type().Elem().Kind() != reflect.Struct {
					panic(fmt.Sprintf("unhandled type %s for key %s", v, key))
				}
				elemVal = reflect.Zero(f.Type().Elem())
			}
			if elemVal.IsValid() {
				fieldVal = reflect.MakeSlice(f.Type(), 0, values.N)
				for i := 0; i < values.N; i++ {
					fieldVal = reflect.Append(fieldVal, elemVal)
				}
			}
		case reflect.Map:
			fieldVal = reflect.MakeMapWithSize(f.Type(), values.N)
			var elemVal reflect.Value
			switch v := f.Interface().(type) {
			case map[string]interface{}, common.MapStr:
				elemVal = reflect.ValueOf(values.Str)
			case map[string]float64:
				elemVal = reflect.ValueOf(values.Float)
			default:
				if f.Type().Elem().Kind() != reflect.Struct {
					panic(fmt.Sprintf("unhandled type %s for key %s", v, key))
				}
				elemVal = reflect.Zero(f.Type().Elem())
			}
			for i := 0; i < values.N; i++ {
				fieldVal.SetMapIndex(reflect.ValueOf(fmt.Sprintf("%s%v", values.Str, i)), elemVal)
			}
		case reflect.Struct:
			switch v := f.Interface().(type) {
			case nullable.String:
				v.Set(values.Str)
				fieldVal = reflect.ValueOf(v)
			case nullable.Int:
				v.Set(values.Int)
				fieldVal = reflect.ValueOf(v)
			case nullable.Interface:
				if strings.Contains(key, "port") {
					v.Set(values.Int)
				} else {
					v.Set(values.Str)
				}
				fieldVal = reflect.ValueOf(v)
			case nullable.Bool:
				v.Set(values.Bool)
				fieldVal = reflect.ValueOf(v)
			case nullable.Float64:
				v.Set(values.Float)
				fieldVal = reflect.ValueOf(v)
			case nullable.TimeMicrosUnix:
				v.Set(values.Time)
				fieldVal = reflect.ValueOf(v)
			case nullable.HTTPHeader:
				v.Set(values.HTTPHeader.Clone())
				fieldVal = reflect.ValueOf(v)
			default:
				if f.IsZero() {
					fieldVal = reflect.Zero(f.Type())
				} else {
					return
				}
			}
		case reflect.Ptr:
			if f.IsNil() {
				fieldVal = reflect.Zero(f.Type())
			}
			return
		default:
			panic(fmt.Sprintf("unhandled type %s for key %s", fKind, key))
		}

		// Run through options, giving an opportunity to disable
		// setting the field, or change the value.
		setField := true
		for _, opt := range opts {
			if !opt(key, f, fieldVal) {
				setField = false
				break
			}
		}
		if setField {
			f.Set(fieldVal)
		}
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
	values *Values) {
	IterateStruct(i, func(f reflect.Value, key string) {
		if isException(key) {
			return
		}
		fVal := f.Interface()
		var newVal interface{}
		switch fVal.(type) {
		case map[string]interface{}:
			m := map[string]interface{}{}
			for i := 0; i < values.N; i++ {
				m[fmt.Sprintf("%s%v", values.Str, i)] = values.Str
			}
			newVal = m
		case common.MapStr:
			m := common.MapStr{}
			for i := 0; i < values.N; i++ {
				m.Put(fmt.Sprintf("%s%v", values.Str, i), values.Str)
			}
			newVal = m
		case []string:
			m := make([]string, values.N)
			for i := 0; i < values.N; i++ {
				m[i] = values.Str
			}
			newVal = m
		case string:
			newVal = values.Str
		case *string:
			newVal = &values.Str
		case int:
			newVal = values.Int
		case *int:
			newVal = &values.Int
		case float64:
			newVal = values.Float
		case *float64:
			val := values.Float
			newVal = &val
		case net.IP:
			newVal = values.IP
		case bool:
			newVal = values.Bool
		case *bool:
			newVal = &values.Bool
		case http.Header:
			newVal = values.HTTPHeader
		case time.Time:
			newVal = values.Time
		default:
			// the populator recursively iterates over struct and structPtr
			// calling this function for all fields;
			// it is enough to only assert they are not zero here
			if f.Type().Kind() == reflect.Struct {
				assert.NotZero(t, fVal, key)
				return
			}
			if f.Type().Kind() == reflect.Ptr && f.Type().Elem().Kind() == reflect.Struct {
				assert.NotZero(t, fVal, key)
				return
			}
			if f.Type().Kind() == reflect.Map || f.Type().Kind() == reflect.Slice {
				assert.NotZero(t, fVal, key)
				return
			}
			panic(fmt.Sprintf("unhandled type %s for key %s", f.Type().Kind(), key))
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
		panic(fmt.Sprintf("iterateStruct: invalid type %s", t.Kind()))
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

		// call the given function with every field
		fn(f, fKey)
		// check field type for recursive iteration
		switch f.Kind() {
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
		case reflect.Map:
			if f.Type().Elem().Kind() != reflect.Struct {
				continue
			}
			iter := f.MapRange()
			for iter.Next() {
				mKey := iter.Key()
				mVal := iter.Value()
				ptr := reflect.New(mVal.Type())
				ptr.Elem().Set(mVal)
				iterateStruct(ptr.Elem(), fmt.Sprintf("%s.[%s]", fKey, mKey), fn)
				f.SetMapIndex(mKey, ptr.Elem())
			}
		case reflect.Slice, reflect.Array:
			if v.Type() == f.Type().Elem() {
				continue
			}
			for j := 0; j < f.Len(); j++ {
				sliceField := f.Index(j)
				switch sliceField.Kind() {
				case reflect.Struct:
					iterateStruct(sliceField, fmt.Sprintf("%s.[%v]", fKey, j), fn)
				case reflect.Ptr:
					if !sliceField.IsZero() && sliceField.Type().Elem().Kind() == reflect.Struct {
						iterateStruct(sliceField.Elem(), fKey, fn)
					}
				}
			}
		}
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
