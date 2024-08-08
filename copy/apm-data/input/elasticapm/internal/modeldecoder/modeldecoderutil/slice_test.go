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
	"testing"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/stretchr/testify/assert"
)

func TestReslice(t *testing.T) {
	var s []*modelpb.APMEvent

	originalSize := 10
	s = Reslice(s, originalSize)
	assert.Equal(t, originalSize, len(s))

	downsize := 4
	s = Reslice(s, downsize)
	assert.Equal(t, downsize, len(s))

	upsize := 21
	s = Reslice(s, upsize)
	assert.Equal(t, upsize, len(s))
}

func TestResliceAndPopulateNil(t *testing.T) {
	var s []*modelpb.APMEvent

	originalSize := 10
	s = ResliceAndPopulateNil(s, originalSize, modelpb.APMEventFromVTPool)
	validateBackingArray(t, s, originalSize)
	assert.Equal(t, originalSize, len(s))

	downsize := 4
	s = ResliceAndPopulateNil(s, downsize, nil)
	validateBackingArray(t, s, downsize)
	assert.Equal(t, downsize, len(s))

	upsize := 21
	s = ResliceAndPopulateNil(s, upsize, modelpb.APMEventFromVTPool)
	validateBackingArray(t, s, upsize)
	assert.Equal(t, upsize, len(s))
}

func validateBackingArray(t *testing.T, out []*modelpb.APMEvent, expectedLen int) {
	t.Helper()

	// validate length
	assert.Equal(t, expectedLen, len(out))

	// validate all elements of backing array are non-nil
	for i := 0; i < len(out); i++ {
		assert.NotNil(t, out[i])
	}
}
