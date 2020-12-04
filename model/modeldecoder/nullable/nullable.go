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

package nullable

import (
	"fmt"
	"net/http"
	"time"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
)

func init() {
	jsoniter.RegisterTypeDecoderFunc("nullable.String", func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
		switch iter.WhatIsNext() {
		case jsoniter.NilValue:
			iter.ReadNil()
		default:
			(*((*String)(ptr))).Val = iter.ReadString()
			(*((*String)(ptr))).isSet = true
		}
	})
	jsoniter.RegisterTypeDecoderFunc("nullable.Int", func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
		switch iter.WhatIsNext() {
		case jsoniter.NilValue:
			iter.ReadNil()
		default:
			(*((*Int)(ptr))).Val = iter.ReadInt()
			(*((*Int)(ptr))).isSet = true
		}
	})
	jsoniter.RegisterTypeDecoderFunc("nullable.Float64", func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
		switch iter.WhatIsNext() {
		case jsoniter.NilValue:
			iter.ReadNil()
		default:
			(*((*Float64)(ptr))).Val = iter.ReadFloat64()
			(*((*Float64)(ptr))).isSet = true
		}
	})
	jsoniter.RegisterTypeDecoderFunc("nullable.Bool", func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
		switch iter.WhatIsNext() {
		case jsoniter.NilValue:
			iter.ReadNil()
		default:
			(*((*Bool)(ptr))).Val = iter.ReadBool()
			(*((*Bool)(ptr))).isSet = true
		}
	})
	jsoniter.RegisterTypeDecoderFunc("nullable.Interface", func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
		switch iter.WhatIsNext() {
		case jsoniter.NilValue:
			iter.ReadNil()
		default:
			v := iter.Read()
			(*((*Interface)(ptr))).Val = v
			(*((*Interface)(ptr))).isSet = true
		}
	})
	jsoniter.RegisterTypeDecoderFunc("nullable.TimeMicrosUnix", func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
		switch iter.WhatIsNext() {
		case jsoniter.NilValue:
			iter.ReadNil()
		default:
			us := iter.ReadInt()
			s := us / 1000000
			ns := (us - (s * 1000000)) * 1000
			(*((*TimeMicrosUnix)(ptr))).Val = time.Unix(int64(s), int64(ns)).UTC()
			(*((*TimeMicrosUnix)(ptr))).isSet = true
		}
	})
	jsoniter.RegisterTypeDecoderFunc("nullable.HTTPHeader", func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
		switch iter.WhatIsNext() {
		case jsoniter.NilValue:
			iter.ReadNil()
		default:
			m, ok := iter.Read().(map[string]interface{})
			if !ok {
				iter.Error = fmt.Errorf("invalid input for HTTPHeader: %v", m)
			}
			h := http.Header{}
			for key, val := range m {
				switch v := val.(type) {
				case nil:
				case string:
					h.Add(key, v)
				case []interface{}:
					for _, entry := range v {
						switch entry := entry.(type) {
						case string:
							h.Add(key, entry)
						default:
							iter.Error = fmt.Errorf("invalid input for HTTPHeader: %v", v)
						}
					}
				default:
					iter.Error = fmt.Errorf("invalid input for HTTPHeader: %v", v)
				}
			}
			(*((*HTTPHeader)(ptr))).Val = h
			(*((*HTTPHeader)(ptr))).isSet = true
		}
	})
}

// String stores a string value and the
// information if the value has been set
type String struct {
	Val   string
	isSet bool
}

// Set sets the value
func (v *String) Set(val string) {
	v.Val = val
	v.isSet = true
}

// IsSet is true when decode was called
func (v *String) IsSet() bool {
	return v.isSet
}

// Reset sets the String to it's initial state
// where it is not set and has no value
func (v *String) Reset() {
	v.Val = ""
	v.isSet = false
}

// Int stores an int value and the
// information if the value has been set
type Int struct {
	Val   int
	isSet bool
}

// Set sets the value
func (v *Int) Set(val int) {
	v.Val = val
	v.isSet = true
}

// IsSet is true when decode was called
func (v *Int) IsSet() bool {
	return v.isSet
}

// Reset sets the Int to it's initial state
// where it is not set and has no value
func (v *Int) Reset() {
	v.Val = 0
	v.isSet = false
}

// Float64 stores a float64 value and the
// information if the value has been set
type Float64 struct {
	Val   float64
	isSet bool
}

// Set sets the value
func (v *Float64) Set(val float64) {
	v.Val = val
	v.isSet = true
}

// IsSet is true when decode was called
func (v *Float64) IsSet() bool {
	return v.isSet
}

// Reset sets the Int to it's initial state
// where it is not set and has no value
func (v *Float64) Reset() {
	v.Val = 0.0
	v.isSet = false
}

// Bool stores a bool value and the
// information if the value has been set
type Bool struct {
	Val   bool
	isSet bool
}

// Set sets the value
func (v *Bool) Set(val bool) {
	v.Val = val
	v.isSet = true
}

// IsSet is true when decode was called
func (v *Bool) IsSet() bool {
	return v.isSet
}

// Reset sets the Int to it's initial state
// where it is not set and has no value
func (v *Bool) Reset() {
	v.Val = false
	v.isSet = false
}

// Interface stores an interface{} value and the
// information if the value has been set
//
// TODO(simitt): follow up on https://github.com/elastic/apm-server/pull/4154#discussion_r484166721
type Interface struct {
	Val   interface{} `json:"val,omitempty"`
	isSet bool
}

// Set sets the value
func (v *Interface) Set(val interface{}) {
	v.Val = val
	v.isSet = true
}

// IsSet is true when decode was called
func (v *Interface) IsSet() bool {
	return v.isSet
}

// Reset sets the Interface to it's initial state
// where it is not set and has no value
func (v *Interface) Reset() {
	v.Val = nil
	v.isSet = false
}

type TimeMicrosUnix struct {
	Val   time.Time
	isSet bool
}

// Set sets the value
func (v *TimeMicrosUnix) Set(val time.Time) {
	v.Val = val
	v.isSet = true
}

// IsSet is true when decode was called
func (v *TimeMicrosUnix) IsSet() bool {
	return v.isSet
}

// Reset sets the Interface to it's initial state
// where it is not set and has no value
func (v *TimeMicrosUnix) Reset() {
	v.Val = time.Time{}
	v.isSet = false
}

type HTTPHeader struct {
	Val   http.Header
	isSet bool
}

// Set sets the value
func (v *HTTPHeader) Set(val http.Header) {
	v.Val = val
	v.isSet = true
}

// IsSet is true when decode was called
func (v *HTTPHeader) IsSet() bool {
	return v.isSet
}

// Reset sets the Interface to it's initial state
// where it is not set and has no value
func (v *HTTPHeader) Reset() {
	for k := range v.Val {
		delete(v.Val, k)
	}
	v.isSet = false
}
