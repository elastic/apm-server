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

package model

import (
	"github.com/elastic/beats/v7/libbeat/common"
)

var (
	// ErrorProcessor is the Processor value that should be assigned to error events.
	ErrorProcessor = Processor{Name: "error", Event: "error"}
)

const (
	ErrorsDataset = "apm.error"
)

type Error struct {
	ID string

	GroupingKey string
	Culprit     string
	Custom      common.MapStr

	// StackTrace holds an unparsed stack trace.
	//
	// This may be set when a stack trace cannot be parsed.
	StackTrace string

	Exception *Exception
	Log       *ErrorLog
}

type Exception struct {
	Message    string
	Module     string
	Code       string
	Attributes interface{}
	Stacktrace Stacktrace
	Type       string
	Handled    *bool
	Cause      []Exception
}

type ErrorLog struct {
	Message      string
	Level        string
	ParamMessage string
	LoggerName   string
	Stacktrace   Stacktrace
}

func (e *Error) fields() common.MapStr {
	var errorFields mapStr
	errorFields.maybeSetString("id", e.ID)
	if e.Exception != nil {
		exceptionFields := e.Exception.appendFields(nil, 0)
		errorFields.set("exception", exceptionFields)
	}
	errorFields.maybeSetMapStr("log", e.logFields())
	errorFields.maybeSetString("culprit", e.Culprit)
	errorFields.maybeSetMapStr("custom", customFields(e.Custom))
	errorFields.maybeSetString("grouping_key", e.GroupingKey)
	errorFields.maybeSetString("stack_trace", e.StackTrace)
	return common.MapStr(errorFields)
}

func (e *Error) logFields() common.MapStr {
	if e.Log == nil {
		return nil
	}
	var log mapStr
	log.maybeSetString("message", e.Log.Message)
	log.maybeSetString("param_message", e.Log.ParamMessage)
	log.maybeSetString("logger_name", e.Log.LoggerName)
	log.maybeSetString("level", e.Log.Level)
	if st := e.Log.Stacktrace.transform(); len(st) > 0 {
		log.set("stacktrace", st)
	}
	return common.MapStr(log)
}

func (e *Exception) appendFields(out []common.MapStr, parentOffset int) []common.MapStr {
	offset := len(out)
	var fields mapStr
	fields.maybeSetString("message", e.Message)
	fields.maybeSetString("module", e.Module)
	fields.maybeSetString("type", e.Type)
	fields.maybeSetString("code", e.Code)
	fields.maybeSetBool("handled", e.Handled)
	if offset > parentOffset+1 {
		// The parent of an exception in the resulting slice is at the offset
		// indicated by the `parent` field (0 index based), or the preceding
		// exception in the slice if the `parent` field is not set.
		fields.set("parent", parentOffset)
	}
	if e.Attributes != nil {
		fields.set("attributes", e.Attributes)
	}
	if n := len(e.Stacktrace); n > 0 {
		frames := make([]common.MapStr, n)
		for i, frame := range e.Stacktrace {
			frames[i] = frame.transform()
		}
		fields.set("stacktrace", frames)
	}
	out = append(out, common.MapStr(fields))
	for _, cause := range e.Cause {
		out = cause.appendFields(out, offset)
	}
	return out
}
