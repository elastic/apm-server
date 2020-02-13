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
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

const addKey = "added"

func TestAddGeneral(t *testing.T) {
	var m common.MapStr
	Set(m, "s", "s")
	assert.Nil(t, m)

	m = common.MapStr{}
	Set(m, "", "")
	assert.Equal(t, common.MapStr{}, m)
}

func TestEmptyCollections(t *testing.T) {
	m := common.MapStr{"foo": "bar", "user": common.MapStr{"id": "1", "name": "bar"}}
	add := common.MapStr{}
	Set(m, "user", add)
	assert.Equal(t, common.MapStr{"foo": "bar"}, m)

	m = common.MapStr{"foo": "bar", "user": common.MapStr{"id": "1", "name": "bar"}}
	add = common.MapStr{"id": nil, "email": nil, "info": common.MapStr{"a": nil}}
	Set(m, "user", add)
	assert.Equal(t, common.MapStr{"foo": "bar"}, m)

	m = common.MapStr{"foo": "bar", "user": common.MapStr{"id": "1", "name": "bar"}}
	add = map[string]interface{}{"id": nil, "email": nil, "info": map[string]interface{}{"a": nil}}
	Set(m, "user", add)
	assert.Equal(t, common.MapStr{"foo": "bar"}, m)

	m = common.MapStr{"foo": "bar", "user": common.MapStr{"id": "1", "name": "bar"}}
	add = map[string]interface{}{}
	Set(m, "user", add)
	assert.Equal(t, common.MapStr{"foo": "bar"}, m)
}

func TestIgnoredTypes(t *testing.T) {
	m := common.MapStr{}

	Set(m, "foo", make(chan int))
	assert.Equal(t, common.MapStr{}, m)

	Set(m, "foo", func() {})
	assert.Equal(t, common.MapStr{}, m)

	uintPtr := uintptr(8)
	Set(m, "foo", uintPtr)
	assert.Equal(t, common.MapStr{}, m)

	a := []int{}
	Set(m, "foo", unsafe.Pointer(&a))
	assert.Equal(t, common.MapStr{}, m)
}

func TestAdd(t *testing.T) {
	existing := "foo"
	newArrMapStr := []common.MapStr{{"b": "bar"}}
	var nilArrMapStr []common.MapStr

	newArrStr := []string{"bar"}
	var nilArrStr []string

	newMap := map[string]interface{}{"b": "bar"}
	var nilMap map[string]interface{}

	newMapStr := common.MapStr{"b": "bar"}
	var nilMapStr common.MapStr

	newStr := "bar"
	var nilStr *string

	newInt := 123
	var nilInt *int

	newBool := true
	var nilBool *bool

	tests := []struct {
		v    interface{}
		expV interface{}
		nilV interface{}
	}{
		{
			v:    "some string",
			expV: "some string",
			nilV: nil,
		},
		{
			v:    &newBool,
			expV: newBool,
			nilV: nilBool,
		},
		{
			v:    &newInt,
			expV: newInt,
			nilV: nilInt,
		},
		{
			v:    &newStr,
			expV: newStr,
			nilV: nilStr,
		},
		{
			v:    newMapStr,
			expV: newMapStr,
			nilV: nilMapStr,
		},
		{
			v:    newMap,
			expV: newMap,
			nilV: nilMap,
		},
		{
			v:    newArrStr,
			expV: newArrStr,
			nilV: nilArrStr,
		},
		{
			v:    newArrMapStr,
			expV: newArrMapStr,
			nilV: nilArrMapStr,
		},
		{
			v:    float64(5.98),
			expV: common.Float(5.980000),
			nilV: nil,
		},
		{
			v:    float32(5.987654321),
			expV: common.Float(float32(5.987654321)),
			nilV: nil,
		},
		{
			v:    float64(5),
			expV: int64(5),
			nilV: nil,
		},
		{
			v:    float32(5),
			expV: int32(5),
			nilV: nil,
		},
	}

	for idx, te := range tests {
		// add new value
		m := common.MapStr{"existing": existing}
		Set(m, addKey, te.v)
		expected := common.MapStr{"existing": existing, addKey: te.expV}
		assert.Equal(t, expected, m,
			fmt.Sprintf("<%v>: Set new value - Expected: %v, Actual: %v", idx, expected, m))

		// replace existing value
		m = common.MapStr{addKey: existing}
		Set(m, addKey, te.v)
		expected = common.MapStr{addKey: te.expV}
		assert.Equal(t, expected, m,
			fmt.Sprintf("<%v>: Replace existing value - Expected: %v, Actual: %v", idx, expected, m))

		// remove empty value
		m = common.MapStr{addKey: existing}
		Set(m, addKey, te.nilV)
		expected = common.MapStr{}
		assert.Equal(t, expected, m,
			fmt.Sprintf("<%v>: Remove empty value - Expected: %v, Actual: %v", idx, expected, m))
	}
}

func TestAddEnsureCopy(t *testing.T) {
	for _, test := range []struct {
		v interface{}
	}{
		{
			common.MapStr{"b": "bar"},
		},
		{
			map[string]interface{}{"b": "bar"},
		},
	} {
		dest := common.MapStr{}
		Set(dest, "key", test.v)

		// modify the original value and ensure if doesn't modify the "add"ed value
		reflect.ValueOf(test.v).SetMapIndex(reflect.ValueOf("f"), reflect.ValueOf("foo"))
		actual := dest["key"]

		assert.NotEqual(t, actual, test.v)
	}
}

func TestDeepAdd(t *testing.T) {
	type M = common.MapStr
	m := M{}
	DeepUpdate(m, "a.b.c", 1)
	DeepUpdate(m, "a.b.d", 2)
	DeepUpdate(m, "a.b.d.3", 3)
	DeepUpdate(m, "a.b.d.4", 4)
	DeepUpdate(m, "a.x.y", 5)
	DeepUpdate(m, "a.x.z.nil", nil)
	DeepUpdate(m, "a.nil", nil)

	assert.Equal(t, M{
		"a": M{
			"b": M{
				"c": 1,
				"d": M{
					"3": 3,
					"4": 4,
				},
			},
			"x": M{
				"y": 5,
			},
		},
	}, m)

	m = M{}
	DeepUpdate(m, "a", 1)
	DeepUpdate(m, "", 2)
	assert.Equal(t, M{"a": 1}, m)
}

func TestMillisAsMicros(t *testing.T) {
	ms := 4.5
	m := MillisAsMicros(ms)
	expectedMap := common.MapStr{"us": 4500}
	assert.Equal(t, expectedMap, m)
}
