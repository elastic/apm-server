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

package api

import "fmt"

// Action are the actions executed on the API.
type Action int

// List of the valid Actions executed by the API.
//go:generate stringer -type=LicenseType -linecomment=true
const (
	Created Action = iota + 1 // created
)

var mapStringToAction = map[string]Action{
	"created": Created,
}

// UnmarshalJSON unmarshal an action string into a constant.
func (a *Action) UnmarshalJSON(b []byte) error {
	k := string(b)
	if len(b) <= 2 {
		return fmt.Errorf(
			"invalid string for action type, received: '%s'",
			k,
		)
	}
	v, found := mapStringToAction[k[1:len(k)-1]]
	if !found {
		return fmt.Errorf(
			"unknown action '%s' returned from the API, valid actions are: 'created'",
			k,
		)
	}
	*a = v
	return nil
}

// BaseResponse the common response from all the API calls.
type BaseResponse struct {
	Action  Action        `json:"action,omitempty"`
	Success bool          `json:"success"`
	Error   ErrorResponse `json:"error,omitempty"`
}

// ErrorResponse contains human readable and machine readable information when an error happens.
type ErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}
