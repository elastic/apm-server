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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type group struct {
	str string
}

func Group(s string) group {
	return group{str: s}
}

// IntPtr is a test helper function that returns the address of the given integer
func IntPtr(i int) *int {
	return &i
}

func strConcat(pre string, post string, delimiter string) string {
	if pre == "" {
		return post
	}
	return pre + delimiter + post
}

func differenceWithGroup(s1 *Set, s2 *Set) *Set {
	s := Difference(s1, s2)

	for _, e2 := range s2.Array() {
		if e2Grp, ok := e2.(group); !ok {
			continue
		} else {
			for _, e1 := range s1.Array() {
				if e1Str, ok := e1.(string); ok {
					if strings.HasPrefix(e1Str, e2Grp.str) {
						s.Remove(e1)
					}
				}
			}

		}
	}
	return s
}

func assertEmptySet(t *testing.T, s *Set, msg string) {
	if s.Len() > 0 {
		assert.Fail(t, msg)
	}
}
