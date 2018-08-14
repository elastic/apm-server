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

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type streamErrorType string

const (
	queueFullErr          streamErrorType = "ERR_QUEUE_FULL"
	processingTimeoutErr  streamErrorType = "ERR_PROCESSING_TIMEOUT"
	schemaValidationErr   streamErrorType = "ERR_SCHEMA_VALIDATION"
	invalidJSONErr        streamErrorType = "ERR_INVALID_JSON"
	shuttingDownErr       streamErrorType = "ERR_SHUTTING_DOWN"
	invalidContentTypeErr streamErrorType = "ERR_CONTENT_TYPE"

	validationErrorsLimit = 5
)

var standardMessages = map[streamErrorType]struct {
	err  string
	code int
}{
	queueFullErr:          {"queue is full", http.StatusTooManyRequests},
	processingTimeoutErr:  {"timeout while waiting to process request", http.StatusRequestTimeout},
	schemaValidationErr:   {"validation error", http.StatusBadRequest},
	invalidJSONErr:        {"invalid JSON", http.StatusBadRequest},
	shuttingDownErr:       {"server is shutting down", http.StatusServiceUnavailable},
	invalidContentTypeErr: {"invalid content-type. Expected 'application/x-ndjson'", http.StatusBadRequest},
}

type streamResponse struct {
	Accepted int `json:"accepted"`
	Invalid  int `json:"invalid"`
	Dropped  int `json:"dropped"`

	Errors map[streamErrorType]errorDetails `json:"errors"`
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

func (s *streamResponse) AddError(errType streamErrorType, count int) {
	if s.Errors == nil {
		s.Errors = make(map[streamErrorType]errorDetails)
	}

	var details errorDetails
	var ok bool
	if details, ok = s.Errors[errType]; !ok {
		s.Errors[errType] = errorDetails{
			Count:   count,
			Message: standardMessages[errType].err,
		}
		return
	}

	details.Count += count
	s.Errors[errType] = details
}

func (s *streamResponse) String() string {
	errorList := []string{}
	for _, t := range []streamErrorType{
		queueFullErr,
		processingTimeoutErr,
		schemaValidationErr,
		invalidJSONErr, shuttingDownErr,
		invalidContentTypeErr,
	} {
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

func (s *streamResponse) StatusCode() int {
	statusCode := http.StatusAccepted
	for k := range s.Errors {
		if standardMessages[k].code > statusCode {
			statusCode = standardMessages[k].code
		}
	}
	return statusCode
}

func (s *streamResponse) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

func (s *streamResponse) ValidationError(err string, offendingDocument string) {
	s.AddError(schemaValidationErr, 0)
	errorDetails := s.Errors[schemaValidationErr]
	if errorDetails.Documents == nil {
		errorDetails.Documents = []*ValidationError{}
		errorDetails.errorsMap = make(map[string]struct{})
	}

	if len(errorDetails.Documents) < validationErrorsLimit {
		// we only want one specimen of each error
		if _, ok := errorDetails.errorsMap[err]; !ok {
			errorDetails.errorsMap[err] = struct{}{}

			errorDetails.Documents = append(errorDetails.Documents, &ValidationError{
				Error:          err,
				OffendingEvent: offendingDocument,
			})
			s.Errors[schemaValidationErr] = errorDetails
		}
	}
}
