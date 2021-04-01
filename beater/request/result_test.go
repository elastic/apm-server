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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/pkg/errors"
)

func assertResultIsEmpty(t *testing.T, r Result) {
	cType := reflect.TypeOf(r)
	cVal := reflect.ValueOf(r)
	for i := 0; i < cVal.NumField(); i++ {
		val := cVal.Field(i).Interface()
		switch cType.Field(i).Name {
		case "ID":
			assert.Equal(t, IDUnset, val)
		case "StatusCode":
			assert.Equal(t, http.StatusOK, val)
		default:
			assert.Empty(t, val)
		}
	}
}

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
	assertResultIsEmpty(t, r)
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
		r.Set(IDUnset, http.StatusBadRequest, MapResultIDToStatus[IDResponseErrorsValidate].Keyword, "foo", nil)
		assert.Equal(t, MapResultIDToStatus[IDResponseErrorsValidate].Keyword, r.Err.Error())
		assert.Equal(t, "foo", r.Body)
	})
	t.Run("SetBodyToKeyword", func(t *testing.T) {
		r := Result{}
		fullQueue := MapResultIDToStatus[IDResponseErrorsFullQueue].Keyword
		r.Set(IDUnset, http.StatusServiceUnavailable, fullQueue, nil, nil)
		assert.Equal(t, fullQueue, r.Err.Error())
		assert.Equal(t, string(fullQueue), r.Body)
	})
	t.Run("SetBodyToErr", func(t *testing.T) {
		err := errors.New("xyz")
		r := Result{}
		r.Set(IDUnset, http.StatusBadRequest, MapResultIDToStatus[IDResponseErrorsDecode].Keyword, nil, err)
		assert.Equal(t, err, r.Err)
		assert.Equal(t, err.Error(), r.Body)
	})
}

func TestResult_SetDefault(t *testing.T) {
	t.Run("StatusAccepted", func(t *testing.T) {
		id := IDResponseValidAccepted
		r := Result{}
		r.Reset()
		r.SetDefault(id)
		assert.Equal(t, id, r.ID)
		assert.Equal(t, http.StatusAccepted, r.StatusCode)
		assert.Equal(t, MapResultIDToStatus[id].Keyword, r.Keyword)
		assert.Equal(t, nil, r.Body)
		assert.Equal(t, nil, r.Err)
		assert.Equal(t, "", r.Stacktrace)
	})

	t.Run("StatusForbidden", func(t *testing.T) {
		id := IDResponseErrorsForbidden
		r := Result{}
		r.Reset()
		r.SetDefault(id)
		assert.Equal(t, id, r.ID)
		assert.Equal(t, http.StatusForbidden, r.StatusCode)
		assert.Equal(t, MapResultIDToStatus[id].Keyword, r.Keyword)
		assert.Equal(t, r.Keyword, r.Body)
		assert.Equal(t, r.Keyword, r.Err.Error())
		assert.Equal(t, "", r.Stacktrace)
	})
}

func TestResult_SetWithBody(t *testing.T) {
	t.Run("StatusAccepted", func(t *testing.T) {
		id := IDResponseValidAccepted
		body := "some body"
		r := Result{}
		r.Reset()
		r.SetWithBody(id, body)
		assert.Equal(t, id, r.ID)
		assert.Equal(t, http.StatusAccepted, r.StatusCode)
		assert.Equal(t, MapResultIDToStatus[id].Keyword, r.Keyword)
		assert.Equal(t, body, r.Body)
		assert.Equal(t, nil, r.Err)
		assert.Equal(t, "", r.Stacktrace)
	})

	t.Run("StatusForbidden", func(t *testing.T) {
		id := IDResponseErrorsForbidden
		body := "some body"
		r := Result{}
		r.Reset()
		r.SetWithBody(id, body)
		assert.Equal(t, id, r.ID)
		assert.Equal(t, http.StatusForbidden, r.StatusCode)
		assert.Equal(t, MapResultIDToStatus[id].Keyword, r.Keyword)
		assert.Equal(t, body, r.Body)
		assert.Equal(t, r.Keyword, r.Err.Error())
		assert.Equal(t, "", r.Stacktrace)
	})
}

func TestResult_SetWithError(t *testing.T) {
	t.Run("StatusAccepted", func(t *testing.T) {
		id := IDResponseValidAccepted
		r := Result{}
		r.Reset()
		r.SetWithError(id, nil)
		assert.Equal(t, id, r.ID)
		assert.Equal(t, http.StatusAccepted, r.StatusCode)
		assert.Equal(t, MapResultIDToStatus[id].Keyword, r.Keyword)
		assert.Equal(t, nil, r.Body)
		assert.Equal(t, nil, r.Err)
		assert.Equal(t, "", r.Stacktrace)
	})

	t.Run("StatusForbidden", func(t *testing.T) {
		id := IDResponseErrorsForbidden
		err := errors.New("some error")
		r := Result{}
		r.Reset()
		r.SetWithError(id, err)
		assert.Equal(t, id, r.ID)
		assert.Equal(t, http.StatusForbidden, r.StatusCode)
		assert.Equal(t, MapResultIDToStatus[id].Keyword, r.Keyword)
		wrappedErr := errors.Wrap(err, r.Keyword).Error()
		assert.Equal(t, wrappedErr, r.Body)
		assert.Equal(t, wrappedErr, r.Err.Error())
		assert.Equal(t, "", r.Stacktrace)
	})
}

func TestResult_Failure(t *testing.T) {
	assert.False(t, (&Result{StatusCode: http.StatusOK}).Failure())
	assert.False(t, (&Result{StatusCode: http.StatusPermanentRedirect}).Failure())
	assert.True(t, (&Result{StatusCode: http.StatusBadRequest}).Failure())
	assert.True(t, (&Result{StatusCode: http.StatusServiceUnavailable}).Failure())
}

func TestDefaultMonitoringMapForRegistry(t *testing.T) {
	mockRegistry := monitoring.Default.NewRegistry("mock-default")
	m := DefaultMonitoringMapForRegistry(mockRegistry)
	assert.Equal(t, 22, len(m))
	for id := range m {
		assert.Equal(t, int64(0), m[id].Get())
	}
}

func TestMonitoringMapForRegistry(t *testing.T) {
	keys := []ResultID{IDEventDroppedCount, IDResponseErrorsServiceUnavailable}
	mockRegistry := monitoring.Default.NewRegistry("mock-with-keys")
	m := MonitoringMapForRegistry(mockRegistry, keys)
	assert.Equal(t, 2, len(m))
	for id := range m {
		assert.Equal(t, int64(0), m[id].Get())
	}
}
