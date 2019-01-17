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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSet(t *testing.T) {
	for _, d := range []struct {
		init []interface{}
		out  []interface{}
	}{
		{[]interface{}{"a", "b", "b"}, []interface{}{"a", "b"}},
		{[]interface{}{1, 2, 1, 0, "a"}, []interface{}{1, 2, 0, "a"}},
		{[]interface{}{}, []interface{}{}},
		{nil, []interface{}{}},
	} {
		assert.ElementsMatch(t, d.out, NewSet(d.init...).Array())
	}
}

func TestSetAdd(t *testing.T) {
	for _, d := range []struct {
		s   *Set
		add interface{}
		out []interface{}
	}{
		{nil, "a", []interface{}{}},
		{NewSet(), "a", []interface{}{"a"}},
		{NewSet(1, "a"), "a", []interface{}{1, "a"}},
		{NewSet(123), "a", []interface{}{123, "a"}},
	} {
		d.s.Add(d.add)
		assert.ElementsMatch(t, d.out, d.s.Array())
	}
}

func TestSetRemove(t *testing.T) {
	for _, d := range []struct {
		s      *Set
		remove interface{}
		out    []interface{}
	}{
		{nil, "a", []interface{}{}},
		{NewSet(), "a", []interface{}{}},
		{NewSet("a"), "a", []interface{}{}},
		{NewSet(123, "a"), 123, []interface{}{"a"}},
		{NewSet(123, "a", "a"), "a", []interface{}{123}},
	} {
		d.s.Remove(d.remove)
		assert.ElementsMatch(t, d.out, d.s.Array())
	}
}

func TestSetContains(t *testing.T) {
	for _, d := range []struct {
		s     *Set
		input interface{}
		out   bool
	}{
		{nil, "a", false},
		{NewSet(), "a", false},
		{NewSet(1, 2, 3), "a", false},
		{NewSet("a", "b"), "a", true},
	} {
		assert.Equal(t, d.out, d.s.Contains(d.input))
	}
}

func TestSetContainsStrPattern(t *testing.T) {
	for _, d := range []struct {
		s     *Set
		input string
		out   bool
	}{
		{NewSet(), "a", false},
		{NewSet(1, 2, 3), "a", false},
		{NewSet("a", 1, "b"), "a", true},
		{NewSet("a.b.c.d", 1, "bc"), "b.c", false},
		{NewSet("b.c", 1, "bc"), "a.b.c", false},
		{NewSet("a.*c.d", 1), "a.b.c.d", true},
		{NewSet("a.*c.", 1), "a.b.c.d", false},
		{NewSet("a.[^.]*.c.d", 1), "a.b_d.c.d", true},
		{NewSet("a.[^.].*c*", 1), "a.b.c.d", true},
		{NewSet("*", 1), "a.b.c.d", true},
		{NewSet("a.[^.].c", 1), "a.b.x.c.d", false},
		{NewSet("metrics.samples.[^.]+.type", "a", 1), "metrics.samples.shortcounter.type", true},
	} {
		assert.Equal(t, d.out, d.s.ContainsStrPattern(d.input))
	}
}

func TestSetCopy(t *testing.T) {
	for _, d := range []struct {
		s   *Set
		out []interface{}
	}{
		{nil, []interface{}{}},
		{NewSet(), []interface{}{}},
		{NewSet("a"), []interface{}{"a"}},
		{NewSet(123, "a", "a"), []interface{}{"a", 123}},
	} {
		copied := d.s.Copy()
		d.s.Add(500)
		assert.ElementsMatch(t, d.out, copied.Array())
	}
}

func TestSetLen(t *testing.T) {
	for _, d := range []struct {
		s   *Set
		len int
	}{
		{nil, 0},
		{NewSet(), 0},
		{NewSet("a"), 1},
		{NewSet(123, "a", "a"), 2},
	} {
		assert.Equal(t, d.len, d.s.Len())
	}
}

func TestSetUnion(t *testing.T) {
	for _, d := range []struct {
		s1  *Set
		s2  *Set
		out []interface{}
	}{
		{nil, nil, []interface{}{}},
		{nil, NewSet(1), []interface{}{1}},
		{NewSet(1), nil, []interface{}{1}},
		{NewSet(), NewSet(), []interface{}{}},
		{NewSet(34.5, "a"), NewSet(), []interface{}{34.5, "a"}},
		{NewSet(), NewSet(1), []interface{}{1}},
		{NewSet(1, 2, 3), NewSet(1, "a"), []interface{}{1, 2, 3, "a"}},
	} {
		assert.ElementsMatch(t, d.out, Union(d.s1, d.s2).Array())
	}
}

func TestSetDifference(t *testing.T) {
	for _, d := range []struct {
		s1  *Set
		s2  *Set
		out []interface{}
	}{
		{nil, nil, []interface{}{}},
		{nil, NewSet("a"), []interface{}{}},
		{NewSet(34.5), nil, []interface{}{34.5}},
		{NewSet(), NewSet(), []interface{}{}},
		{NewSet(34.5, "a"), NewSet(), []interface{}{34.5, "a"}},
		{NewSet(), NewSet(1), []interface{}{}},
		{NewSet(1, 2, 3), NewSet(1, "a"), []interface{}{2, 3}},
	} {
		assert.ElementsMatch(t, d.out, Difference(d.s1, d.s2).Array())
	}
}

func TestSetSymmDifference(t *testing.T) {
	for _, d := range []struct {
		s1  *Set
		s2  *Set
		out []interface{}
	}{
		{nil, nil, []interface{}{}},
		{nil, NewSet("a"), []interface{}{"a"}},
		{NewSet("a"), nil, []interface{}{"a"}},
		{NewSet(34.5), nil, []interface{}{34.5}},
		{NewSet(), NewSet(), []interface{}{}},
		{NewSet(34.5, "a"), NewSet(), []interface{}{34.5, "a"}},
		{NewSet(), NewSet(1), []interface{}{1}},
		{NewSet(1, 2, 3, 8.9, "b"), NewSet(1, "a", 8.9, "b"), []interface{}{2, 3, "a"}},
	} {
		assert.ElementsMatch(t, d.out, SymmDifference(d.s1, d.s2).Array())
	}
}

func TestSetArray(t *testing.T) {
	for _, d := range []struct {
		s   *Set
		out []interface{}
	}{
		{nil, []interface{}{}},
		{NewSet(), []interface{}{}},
		{NewSet(34.5, "a"), []interface{}{34.5, "a"}},
		{NewSet(1, 1, 1), []interface{}{1}},
	} {
		assert.ElementsMatch(t, d.out, d.s.Array())
	}
}
