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
	jsoniter.RegisterTypeDecoderFunc("nullable.Interface", func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
		switch iter.WhatIsNext() {
		case jsoniter.NilValue:
			iter.ReadNil()
		default:
			(*((*Interface)(ptr))).Val = iter.Read()
			(*((*Interface)(ptr))).isSet = true
		}
	})
}

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
