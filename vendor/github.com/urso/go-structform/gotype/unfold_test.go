package gotype

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/urso/go-structform/json"
)

var unfoldSamples = []struct {
	json  string
	input interface{}
	value interface{}
}{

	// primitives
	{`null`, nil, new(interface{})},
	{`""`, nil, new(string)},
	{`true`, true, new(bool)},
	{`true`, true, new(interface{})},
	{`false`, false, new(bool)},
	{`false`, false, new(interface{})},
	{`null`, nil, new(*int)},
	{`10`, int8(10), new(int8)},
	{`10`, int8(10), new(int)},
	{`10`, int8(10), new(*int8)},
	{`10`, int8(10), new(*int)},
	{`10`, int8(10), new(interface{})},
	{`10`, int32(10), new(int64)},
	{`10`, int32(10), new(int16)},
	{`10`, int32(10), new(interface{})},
	{`10`, int(10), new(int)},
	{`10`, int(10), new(uint)},
	{`10`, uint(10), new(uint)},
	{`10`, uint(10), new(uint16)},
	{`10`, uint(10), new(interface{})},
	{`10`, uint8(10), new(int)},
	{`10`, uint16(10), new(uint64)},
	{`10`, uint32(10), new(uint8)},
	{`12340`, uint16(12340), new(uint16)},
	{`12340`, uint16(12340), new(interface{})},
	{`1234567`, uint32(1234567), new(uint32)},
	{`1234567`, uint32(1234567), new(int64)},
	{`12345678190`, uint64(12345678190), new(uint64)},
	{`12345678190`, uint64(12345678190), new(int64)},
	{`-10`, int8(-10), new(int)},
	{`-10`, int8(-10), new(int8)},
	{`-10`, int8(-10), new(int32)},
	{`-10`, int8(-10), new(int64)},
	{`-10`, int8(-10), new(interface{})},
	{`3.14`, float32(3.14), new(float32)},
	{`3.14`, float32(3.14), new(interface{})},
	{`3.14`, float64(3.14), new(float32)},
	{`3.14`, float64(3.14), new(float64)},
	{`3.14`, float64(3.14), new(interface{})},
	{`"test"`, "test", new(string)},
	{`"test"`, "test", new(interface{})},
	{`"test with \" being escaped"`, "test with \" being escaped", new(string)},
	{`"test with \" being escaped"`, "test with \" being escaped", new(interface{})},

	// arrays
	{`[]`, []uint8{}, new(interface{})},
	{`[]`, []uint8{}, new([]uint8)},
	{`[]`, []interface{}{}, new([]interface{})},
	// {`[]`, []interface{}{}, &[]struct{ A string }{}},
	{`[1,2,3]`, []uint8{1, 2, 3}, new(interface{})},
	{`[1,2,3]`, []uint8{1, 2, 3}, &[]uint8{0, 0, 0}},
	{`[1,2,3]`, []uint8{1, 2, 3}, &[]uint8{0, 0, 0, 4}},
	{`[1,2,3]`, []uint8{1, 2, 3}, &[]uint8{}},
	{`[1,2,3]`, []interface{}{1, 2, 3}, &[]uint8{}},
	{`[1,2,3]`, []interface{}{1, 2, 3}, new([]uint8)},
	{`[1,2,3]`, []int{1, 2, 3}, &[]interface{}{}},
	{`[1,2,3]`, []int{1, 2, 3}, &[]interface{}{nil, nil, nil, 4}},
	{`[1]`, []uint8{1}, &[]uint8{0, 2, 3}},
	{`[1,2]`, []uint8{1, 2}, &[]uint8{0, 0, 3}},
	{`[1,2]`, []interface{}{1, 2}, &[]uint{0, 0, 3}},
	{`[1,2]`, []interface{}{1, 2}, &[]interface{}{0, 0, 3}},
	{`["a","b"]`, []string{"a", "b"}, &[]interface{}{}},
	{`["a","b"]`, []string{"a", "b"}, &[]interface{}{nil, nil}},
	{`["a","b"]`, []string{"a", "b"}, &[]string{}},
	{`["a","b"]`, []string{"a", "b"}, new([]string)},
	{`[null,true,false,123,3.14,"test"]`,
		[]interface{}{nil, true, false, 123, 3.14, "test"},
		new(interface{})},
	{`[null,true,false,123,3.14,"test"]`,
		[]interface{}{nil, true, false, 123, 3.14, "test"},
		&[]interface{}{}},
	{`[1,2,3]`, []int{1, 2, 3}, &[]*int{}},
	{`[1,null,3]`, []interface{}{1, nil, 3}, &[]*int{}},

	// nested arrays
	{`[[]]`, []interface{}{[]uint{}}, new(interface{})},
	{`[]`, []interface{}{}, &[]interface{}{[]interface{}{1}}},
	{`[]`, []interface{}{}, &[][]int{}},
	{`[]`, []interface{}{}, &[][]int{{1}}},
	{`[[1]]`, []interface{}{[]interface{}{1}}, &[][]int{}},
	{`[[1,2,3],[4,5,6]]`,
		[]interface{}{[]interface{}{1, 2, 3}, []interface{}{4, 5, 6}},
		new(interface{})},
	{`[[1],[4]]`,
		[]interface{}{[]interface{}{1}, []interface{}{4}},
		&[]interface{}{[]interface{}{0, 2, 3}}},
	{`[[1],[4],[6]]`,
		[]interface{}{[]interface{}{1}, []interface{}{4}, []interface{}{6}},
		&[]interface{}{
			[]interface{}{0, 2, 3},
			[]interface{}{0, 5},
		}},
	{`[[1],[4],[6]]`,
		[]interface{}{[]interface{}{1}, []interface{}{4}, []interface{}{6}},
		&[][]int{
			{0, 2, 3},
			{0, 5},
		},
	},

	// maps
	{`{}`, map[string]interface{}{}, new(interface{})},
	{`{}`, map[string]interface{}{}, &map[string]interface{}{}},
	{`{"a":1}`, map[string]int{"a": 1}, new(interface{})},
	{`{"a":1}`, map[string]int{"a": 1}, &map[string]interface{}{}},
	{`{"a":1}`, map[string]int{"a": 1}, &map[string]interface{}{"a": 2}},
	{`{"a":1}`, map[string]int{"a": 1}, &map[string]int{}},
	{`{"a":1}`, map[string]int{"a": 1}, &map[string]int{"a": 2}},
	{`{"a":1}`, struct{ A int }{1}, &map[string]interface{}{}},
	{`{"a":1}`, struct{ A int }{1}, &map[string]int{}},
	{`{"a":1,"b":"b","c":true}`,
		map[string]interface{}{"a": 1, "b": "b", "c": true},
		new(interface{}),
	},

	// nested maps
	{`{"a":{}}`, map[string]interface{}{"a": map[string]interface{}{}}, new(interface{})},
	{`{"a":{}}`,
		map[string]interface{}{"a": map[string]interface{}{}},
		&map[string]interface{}{}},
	{`{"a":{}}`,
		map[string]interface{}{"a": map[string]interface{}{}},
		&map[string]interface{}{"a": nil}},
	{`{"a":{}}`,
		map[string]interface{}{"a": map[string]interface{}{}},
		&map[string]map[string]string{"a": nil}},
	{`{"0":{"a":1,"b":2,"c":3},"1":{"e":5,"f":6}}`,
		map[string]map[string]int{
			"0": {"a": 1, "b": 2, "c": 3},
			"1": {"e": 5, "f": 6},
		},
		new(interface{})},
	{`{"0":{"a":1,"b":2,"c":3},"1":{"e":5,"f":6}}`,
		map[string]map[string]int{
			"0": {"a": 1, "b": 2, "c": 3},
			"1": {"e": 5, "f": 6},
		},
		&map[string]interface{}{}},
	{`{"0":{"a":1,"b":2,"c":3},"1":{"e":5,"f":6}}`,
		map[string]map[string]int{
			"0": {"a": 1, "b": 2, "c": 3},
			"1": {"e": 5, "f": 6},
		},
		&map[string]map[string]int64{}},
	{`{"0":{"a":1}}`,
		map[string]map[string]int{
			"0": {"a": 1},
		},
		&map[string]interface{}{
			"0": map[string]int{"b": 2, "c": 3},
		}},
	{`{"0":{"a":1},"1":{"e":5}}`,
		map[string]map[string]int{
			"0": {"a": 1},
			"1": {"e": 5},
		},
		&map[string]interface{}{
			"0": map[string]int{"b": 2, "c": 3},
			"1": map[string]interface{}{"f": 6, "e": 0},
		}},
	{`{"0":{"a":1},"1":{"e":5}}`,
		map[string]map[string]int{
			"0": {"a": 1},
			"1": {"e": 5},
		},
		&map[string]map[string]uint{
			"0": {"b": 2, "c": 3},
			"1": {"f": 6, "e": 0},
		}},

	// map in array
	{`[{}]`, []interface{}{map[string]interface{}{}}, new(interface{})},
	{`[{}]`, []interface{}{map[string]interface{}{}}, &[]map[string]int{}},
	{`[{"a":1}]`, []map[string]int{{"a": 1}}, new(interface{})},
	{`[{"a":1}]`, []map[string]int{{"a": 1}}, &[]interface{}{}},
	{`[{"a":1}]`, []map[string]int{{"a": 1}}, &[]map[string]interface{}{}},
	{`[{"a":1}]`, []map[string]int{{"a": 1}}, &[]map[string]interface{}{{"a": 2}}},
	{`[{"a":1,"b":2}]`, []map[string]int{{"a": 1}}, &[]map[string]int{{"b": 2}}},
	{`[{"a":1},{"b":2}]`, []map[string]int{{"a": 1}, {"b": 2}}, &[]map[string]int{}},
	{`[{"a":1},{"b":"b"},{"c":true}]`,
		[]map[string]interface{}{{"a": 1}, {"b": "b"}, {"c": true}},
		new(interface{})},

	// array in map
	{`{"a": []}`, map[string]interface{}{"a": []int{}}, new(interface{})},
	{`{"a":[1,2],"b":[3]}`, map[string][]int{"a": {1, 2}, "b": {3}}, new(interface{})},
	{`{"a":[1,2],"b":[3]}`, map[string][]int{"a": {1, 2}, "b": {3}},
		&map[string][]int{}},
	{`{"a":[1,2],"b":[3,4,5]}`, map[string][]int{"a": {1, 2}, "b": {3, 4, 5}},
		&map[string][]int{"a": {0, 2}, "b": {0}}},

	// struct
	{`{"a":1}`, map[string]int{"a": 1}, &struct{ A int }{}},
	{`{"a":1}`, map[string]int{"a": 1}, &struct{ A *int }{}},
	{`{"a": 1, "c": 2}`, map[string]int{"a": 1}, &struct{ A, b, C int }{b: 1, C: 2}},
	{`{"a": {"c": 2}, "b": 1}`,
		map[string]interface{}{"a": map[string]int{"c": 2}},
		&struct {
			A struct{ C int }
			B int
		}{B: 1},
	},
	{`{"a": 1}`,
		map[string]interface{}{"a": 1},
		&struct {
			S struct {
				A int
			} `struct:",inline"`
		}{},
	},
	{`{"a":{"b":{"c":1}}}`,
		map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]int{
					"c": 1,
				},
			},
		},
		&struct{ A struct{ B struct{ C int } } }{},
	},
}

func TestFoldUnfoldConsistent(t *testing.T) {
	tests := unfoldSamples
	for i, test := range tests {
		t.Logf("run test (%v): %v (%T -> %T)", i, test.json, test.input, test.value)

		u, err := NewUnfolder(test.value)
		if err != nil {
			t.Errorf("NewUnfolder failed with: %v", err)
			continue
		}

		if err := Fold(test.input, u); err != nil {
			t.Errorf("Fold-Unfold failed with: %v", err)
			continue
		}

		if st := &u.unfolder; len(st.stack) > 0 {
			t.Errorf("Unfolder state stack not empty: %v, %v", st.stack, st.current)
			continue
		}

		// serialize to json
		var buf bytes.Buffer
		if err := Fold(test.value, json.NewVisitor(&buf)); err != nil {
			t.Errorf("serialize to json failed with: %v", err)
			continue
		}

		// compare conversions did preserve type
		assertJSON(t, test.json, buf.String())
	}
}

func TestUnfoldJsonInto(t *testing.T) {
	tests := unfoldSamples
	for i, test := range tests {
		t.Logf("run test (%v): %v (%T -> %T)", i, test.json, test.input, test.value)

		un, err := NewUnfolder(test.value)
		if err != nil {
			t.Fatal(err)
		}

		dec := json.NewParser(un)
		input := test.json

		err = dec.ParseString(input)
		if err != nil {
			t.Error(err)
			continue
		}

		// check state valid by processing a second time
		if err = un.SetTarget(test.value); err != nil {
			t.Error(err)
			continue
		}

		err = dec.ParseString(input)
		if err != nil {
			t.Error(err)
			continue
		}
	}
}

func BenchmarkUnfoldJsonInto(b *testing.B) {
	tests := unfoldSamples
	for i, test := range tests {
		name := fmt.Sprintf("%v (%T->%T)", test.json, test.input, test.value)
		b.Logf("run test (%v): %v", i, name)

		un, err := NewUnfolder(test.value)
		if err != nil {
			b.Fatal(err)
		}

		dec := json.NewParser(un)
		input := test.json
		// parse once to reset state for use of 'setTarget' in benchmark
		if err := dec.ParseString(input); err != nil {
			b.Error(err)
			continue
		}

		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				un.SetTarget(test.value)
				err := dec.ParseString(input)
				if err != nil {
					b.Error(err)
				}

			}
		})
	}
}
