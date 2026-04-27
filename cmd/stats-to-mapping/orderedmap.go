// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// orderedMap is an ordered key/value collection that preserves insertion
// order across JSON unmarshal and marshal.
//
// Values are stored as any so the structure can hold heterogeneous JSON
// trees (other *orderedMap, []any, json.Number, string, bool, nil).
type orderedMap struct {
	keys   []string
	values map[string]any
}

func newOrderedMap() *orderedMap {
	return &orderedMap{values: map[string]any{}}
}

// Set inserts or updates a key, preserving original insertion order on update.
func (m *orderedMap) Set(key string, value any) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Get returns the value for key and whether it was present.
func (m *orderedMap) Get(key string) (any, bool) {
	v, ok := m.values[key]
	return v, ok
}

// MarshalJSON emits keys in insertion order. Values are marshaled with the
// standard library encoder, with HTML escaping disabled to match Python's
// json.dumps(ensure_ascii=False)-equivalent default behavior for our inputs.
func (m *orderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := marshalValue(m.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON parses a JSON object into m, preserving key order. Numbers
// are decoded as json.Number so their textual form (and integer-ness)
// round-trips on re-marshal.
func (m *orderedMap) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return err
	}
	src, ok := v.(*orderedMap)
	if !ok {
		return fmt.Errorf("orderedMap: expected JSON object, got %T", v)
	}
	*m = *src
	return nil
}

// decodeValue reads the next JSON value from dec. Objects become
// *orderedMap (preserving key order); arrays become []any; scalars are
// returned as-is from the decoder.
func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch d {
	case '{':
		obj := newOrderedMap()
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyTok.(string)
			if !ok {
				return nil, fmt.Errorf("expected string key, got %v", keyTok)
			}
			v, err := decodeValue(dec)
			if err != nil {
				return nil, fmt.Errorf("decoding value for %q: %w", key, err)
			}
			obj.Set(key, v)
		}
		_, err := dec.Token() // closing '}'
		return obj, err
	case '[':
		arr := []any{}
		for dec.More() {
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		_, err := dec.Token() // closing ']'
		return arr, err
	default:
		return nil, fmt.Errorf("unexpected delim %v", d)
	}
}

// marshalValue serializes a value tree, dispatching to orderedMap.MarshalJSON
// for *orderedMap children. HTML escaping is always off.
func marshalValue(v any) ([]byte, error) {
	switch x := v.(type) {
	case *orderedMap:
		return x.MarshalJSON()
	case []any:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			b, err := marshalValue(e)
			if err != nil {
				return nil, err
			}
			buf.Write(b)
		}
		buf.WriteByte(']')
		return buf.Bytes(), nil
	default:
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return nil, err
		}
		// json.Encoder.Encode appends a newline; strip it.
		return bytes.TrimRight(buf.Bytes(), "\n"), nil
	}
}

// indentJSON formats compact JSON to match Python's json.dumps(..., indent=2).
// stdlib encoding/json.Indent produces an identical layout for our inputs:
// 2-space indent, ": " between key and value, ",\n" between items, and
// compact "{}" / "[]" for empty containers.
func indentJSON(compact []byte, indent string) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, compact, "", indent); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
