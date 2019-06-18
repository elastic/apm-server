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

package server

import (
	"errors"
	"net/http"
)

// Response abstracts an HTTP response
type Response interface {
	Body() interface{}
	Code() int
	IsError() bool
}

type Result struct {
	StatusCode   int
	ResponseBody interface{}
}

func (r Result) Code() int {
	return r.StatusCode
}

func (r Result) IsError() bool {
	return r.StatusCode > 299
}

func (r Result) Body() interface{} {
	return r.ResponseBody
}

// Error abstracts an HTTP error response
type Error struct {
	Err        error
	StatusCode int
}

func (e Error) Code() int {
	return e.StatusCode
}

func (e Error) IsError() bool {
	return true
}

func (e Error) Body() interface{} {
	if e.Err == nil {
		return nil
	}
	return map[string]interface{}{"error": e.Err.Error()}
}

func Unauthorized() *Error {
	return &Error{errors.New("invalid token"),
		http.StatusUnauthorized}
}

func RateLimited() *Error {
	return &Error{errors.New("rate limit exceeded: too many requests"),
		http.StatusTooManyRequests}
}

func MethodNotAllowed() *Error {
	return &Error{errors.New("HTTP method not allowed"),
		http.StatusMethodNotAllowed}
}

func ShuttingDown() *Error {
	return &Error{errors.New("server is shutting down"),
		http.StatusServiceUnavailable}
}
