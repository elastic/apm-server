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

package beater

type streamResponse struct {
	Errors   map[int]map[string]uint `json:"errors"`
	Accepted uint                    `json:"accepted"`
	Invalid  uint                    `json:"invalid"`
	Dropped  uint                    `json:"dropped"`
}

func (s *streamResponse) addErrorCount(serverResponse serverResponse, count int) {
	if s.Errors == nil {
		s.Errors = make(map[int]map[string]uint)
	}

	errorMsgs, ok := s.Errors[serverResponse.code]
	if !ok {
		s.Errors[serverResponse.code] = make(map[string]uint)
		errorMsgs = s.Errors[serverResponse.code]
	}
	errorMsgs[serverResponse.err.Error()] += uint(count)
}

func (s *streamResponse) addError(serverResponse serverResponse) {
	s.addErrorCount(serverResponse, 1)
}
