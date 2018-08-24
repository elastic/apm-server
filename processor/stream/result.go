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
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type StreamErrorType string

const (
	QueueFullErr          StreamErrorType = "ERR_QUEUE_FULL"
	ProcessingTimeoutErr  StreamErrorType = "ERR_PROCESSING_TIMEOUT"
	SchemaValidationErr   StreamErrorType = "ERR_SCHEMA_VALIDATION"
	InvalidJSONErr        StreamErrorType = "ERR_INVALID_JSON"
	ShuttingDownErr       StreamErrorType = "ERR_SHUTTING_DOWN"
	InvalidContentTypeErr StreamErrorType = "ERR_CONTENT_TYPE"
	ServerError           StreamErrorType = "ERR_SERVER_ERROR"

	validationErrorsLimit = 5
)

var standardMessages = map[StreamErrorType]struct {
	err  string
	code int
}{
	QueueFullErr:          {"queue is full", http.StatusTooManyRequests},
	ProcessingTimeoutErr:  {"timeout while waiting to process request", http.StatusRequestTimeout},
	SchemaValidationErr:   {"validation error", http.StatusBadRequest},
	InvalidJSONErr:        {"invalid JSON", http.StatusBadRequest},
	ShuttingDownErr:       {"server is shutting down", http.StatusServiceUnavailable},
	InvalidContentTypeErr: {"invalid content-type. Expected 'application/x-ndjson'", http.StatusBadRequest},
	ServerError:           {"internal server error", http.StatusInternalServerError},
}

var errorTypesDecreasingHTTPStatus = func() []StreamErrorType {
	keys := []StreamErrorType{}
	for k := range standardMessages {
		keys = append(keys, k)
	}

	// sort in reverse order
	sort.Slice(keys, func(i, j int) bool { return standardMessages[keys[i]].code > standardMessages[keys[j]].code })

	return keys
}()

type Result struct {
	Accepted int `json:"accepted"`
	Invalid  int `json:"invalid"`
	Dropped  int `json:"dropped"`

	Errors map[StreamErrorType]errorDetails `json:"errors"`
}

type errorDetails struct {
	Count   int    `json:"count"`
	Message string `json:"message"`

	Documents []*ValidationError `json:"documents,omitempty"`

	// we only use this for deduplication
	errorsMap map[string]struct{}
}

type ValidationError struct {
	Error          string `json:"error"`
	OffendingEvent string `json:"object"`
}

func (s *Result) Add(err StreamErrorType, count int) {
	s.AddWithMessage(err, count, standardMessages[err].err)
}

func (s *Result) AddWithMessage(err StreamErrorType, count int, message string) {
	if s.Errors == nil {
		s.Errors = make(map[StreamErrorType]errorDetails)
	}

	var details errorDetails
	var ok bool
	if details, ok = s.Errors[err]; !ok {
		s.Errors[err] = errorDetails{
			Count:   count,
			Message: message,
		}
		return
	}

	details.Count += count
	s.Errors[err] = details
}

func (s *Result) String() string {
	errorList := []string{}
	for _, t := range errorTypesDecreasingHTTPStatus {
		if s.Errors[t].Count > 0 {
			errorStr := fmt.Sprintf("%s (%d)", s.Errors[t].Message, s.Errors[t].Count)

			if len(s.Errors[t].Documents) > 0 {
				errorStr += ": "
				var docsErrorList []string
				for _, d := range s.Errors[t].Documents {
					docsErrorList = append(docsErrorList, fmt.Sprintf("%s (%s)", d.Error, d.OffendingEvent))
				}
				errorStr += strings.Join(docsErrorList, ", ")
			}

			errorList = append(errorList, errorStr)
		}
	}
	return strings.Join(errorList, ", ")
}

func (s *Result) StatusCode() int {
	statusCode := http.StatusAccepted
	for k := range s.Errors {
		if standardMessages[k].code > statusCode {
			statusCode = standardMessages[k].code
		}
	}
	return statusCode
}

func (s *Result) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

func (s *Result) AddWithOffendingDocument(errType StreamErrorType, errMsg string, offendingDocument []byte) {
	s.Add(errType, 1)
	errorDetails := s.Errors[errType]
	if errorDetails.Documents == nil {
		errorDetails.Documents = []*ValidationError{}
		errorDetails.errorsMap = make(map[string]struct{})
	}

	if len(errorDetails.Documents) < validationErrorsLimit {
		// we only want one specimen of each error
		if _, ok := errorDetails.errorsMap[errMsg]; !ok {
			errorDetails.errorsMap[errMsg] = struct{}{}

			errorDetails.Documents = append(errorDetails.Documents, &ValidationError{
				Error:          errMsg,
				OffendingEvent: string(offendingDocument),
			})
			s.Errors[errType] = errorDetails
		}
	}
}
