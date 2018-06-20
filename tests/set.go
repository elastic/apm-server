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

package tests

type Set struct {
	entries map[interface{}]interface{}
}

func NewSet(entries ...interface{}) *Set {
	s := Set{entries: map[interface{}]interface{}{}}
	for _, v := range entries {
		s.Add(v)
	}
	return &s
}

func (s *Set) Add(input interface{}) {
	s.entries[input] = nil
}

func (s *Set) Remove(input interface{}) {
	delete(s.entries, input)
}

func (s *Set) Contains(input interface{}) bool {
	if _, ok := s.entries[input]; ok {
		return true
	}
	return false
}

func (s *Set) Copy() *Set {
	copy := NewSet()
	for k, _ := range s.entries {
		copy.Add(k)
	}
	return copy
}

func (s *Set) Len() int {
	return len(s.entries)
}

func Union(s1, s2 *Set) *Set {
	s := s1.Copy()
	for k, _ := range s2.entries {
		s.Add(k)
	}
	return s
}

func Difference(s1, s2 *Set) *Set {
	s := NewSet()
	for k, _ := range s1.entries {
		if !s2.Contains(k) {
			s.Add(k)
		}
	}
	return s
}

func SymmDifference(s1, s2 *Set) *Set {
	return Union(Difference(s1, s2), Difference(s2, s1))
}

func (s *Set) Array() []interface{} {
	if s == nil {
		return nil
	}
	a := make([]interface{}, 0, len(s.entries))
	for k, _ := range s.entries {
		a = append(a, k)
	}
	return a
}
