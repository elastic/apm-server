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

package stream

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamResponseSimple(t *testing.T) {
	sr := Result{}
	sr.LimitedAdd(&Error{Type: QueueFullErrType, Message: "err1", Document: "buf1"})
	sr.LimitedAdd(errors.New("transmogrifier error"))
	sr.LimitedAdd(&Error{Type: InvalidInputErrType, Message: "err2", Document: "buf2"})
	sr.LimitedAdd(&Error{Type: InvalidInputErrType, Message: "err3", Document: "buf3"})

	sr.LimitedAdd(&Error{Message: "err4"})
	sr.LimitedAdd(&Error{Message: "err5"})

	// not added
	sr.LimitedAdd(&Error{Message: "err6"})

	// added
	sr.Add(&Error{Message: "err6"})

	assert.Len(t, sr.Errors, 6)

	expectedStr := `err1 [buf1], transmogrifier error, err2 [buf2], err3 [buf3], err4, err6`
	assert.Equal(t, expectedStr, sr.String())
}
