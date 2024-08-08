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

package modeldecoderutil

import (
	"slices"
)

// Reslice returns a slice with length n. It will reallocate the
// slice if the capacity is not enough.
func Reslice[Slice ~[]model, model any](slice Slice, n int) Slice {
	if n > cap(slice) {
		slice = slices.Grow(slice, n-len(slice))
	}
	return slice[:n]
}

// ResliceAndPopulateNil ensures a slice of pointers has atleast
// capacity for n elements and populates any non-nil elements
// in the resulting slice with the value returned from newFn.
func ResliceAndPopulateNil[Slice ~[]*model, model any](slice Slice, n int, newFn func() *model) Slice {
	slice = Reslice(slice, n)
	if newFn != nil {
		for i := 0; i < len(slice); i++ {
			if slice[i] == nil {
				slice[i] = newFn()
			}
		}
	}
	return slice
}
