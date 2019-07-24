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

package request

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pkg/errors"
)

func TestResult_Reset(t *testing.T) {
	r := Result{
		ID:         IDResponseErrorsInternal,
		StatusCode: http.StatusServiceUnavailable,
		Keyword:    "some keyword",
		Body:       []interface{}{1, "foo"},
		Err:        errors.New("foo"),
		Stacktrace: "bar",
	}
	r.Reset()
	assert.Equal(t, r.ID, IDUnset)
	assert.Equal(t, http.StatusOK, r.StatusCode)
	for _, prop := range []interface{}{r.Keyword, r.Body, r.Err, r.Stacktrace} {
		assert.Empty(t, prop)
	}
}

func TestResult_Set(t *testing.T) {
	t.Run("StatusOK", func(t *testing.T) {
		id, statusCode, keyword := IDRequestCount, http.StatusMultipleChoices, "multiple"
		body, err, stacktrace := "foo", errors.New("bar"), "x: bar"
		r := Result{Stacktrace: stacktrace}
		r.Set(id, statusCode, keyword, body, err)
		assert.Equal(t, id, r.ID)
		assert.Equal(t, statusCode, r.StatusCode)
		assert.Equal(t, keyword, r.Keyword)
		assert.Equal(t, body, r.Body)
		assert.Equal(t, err, r.Err)
		assert.Equal(t, stacktrace, r.Stacktrace)
	})

	t.Run("SetErrToKeyword", func(t *testing.T) {
		r := Result{}
		r.Set(IDUnset, http.StatusBadRequest, KeywordResponseErrorsValidate, "foo", nil)
		assert.Equal(t, KeywordResponseErrorsValidate, r.Err.Error())
		assert.Equal(t, "foo", r.Body)
	})
	t.Run("SetBodyToKeyword", func(t *testing.T) {
		r := Result{}
		r.Set(IDUnset, http.StatusServiceUnavailable, KeywordResponseErrorsFullQueue, nil, nil)
		assert.Equal(t, KeywordResponseErrorsFullQueue, r.Err.Error())
		assert.Equal(t, string(KeywordResponseErrorsFullQueue), r.Body)
	})
	t.Run("SetBodyToErr", func(t *testing.T) {
		err := errors.New("xyz")
		r := Result{}
		r.Set(IDUnset, http.StatusBadRequest, KeywordResponseErrorsDecode, nil, err)
		assert.Equal(t, err, r.Err)
		assert.Equal(t, err.Error(), r.Body)
	})
}

func TestResult_Failure(t *testing.T) {
	assert.False(t, (&Result{StatusCode: http.StatusOK}).Failure())
	assert.False(t, (&Result{StatusCode: http.StatusPermanentRedirect}).Failure())
	assert.True(t, (&Result{StatusCode: http.StatusBadRequest}).Failure())
	assert.True(t, (&Result{StatusCode: http.StatusServiceUnavailable}).Failure())
}
