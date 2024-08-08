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

package modeljson

import (
	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
)

func ErrorModelJSON(e *modelpb.Error, out *modeljson.Error) {
	*out = modeljson.Error{
		ID:          e.Id,
		GroupingKey: e.GroupingKey,
		Culprit:     e.Culprit,
		Message:     e.Message,
		Type:        e.Type,
		StackTrace:  e.StackTrace,
	}
	if e.Custom != nil {
		updateFields(e.Custom)
		out.Custom = e.Custom
	}
	if e.Exception != nil {
		out.Exception = &modeljson.Exception{}
		ExceptionModelJSON(e.Exception, out.Exception)
	}
	if e.Log != nil {
		out.Log = &modeljson.ErrorLog{
			Message:      e.Log.Message,
			ParamMessage: e.Log.ParamMessage,
			LoggerName:   e.Log.LoggerName,
			Level:        e.Log.Level,
		}
		if n := len(e.Log.Stacktrace); n > 0 {
			out.Log.Stacktrace = make([]modeljson.StacktraceFrame, n)
			for i, frame := range e.Log.Stacktrace {
				if frame != nil {
					StacktraceFrameModelJSON(frame, &out.Log.Stacktrace[i])
				}
			}
		}
	}
}

func ExceptionModelJSON(e *modelpb.Exception, out *modeljson.Exception) {
	*out = modeljson.Exception{
		Message: e.Message,
		Module:  e.Module,
		Code:    e.Code,
		Type:    e.Type,
		Handled: e.Handled,
	}
	if e.Attributes != nil {
		out.Attributes = e.Attributes
	}
	if n := len(e.Cause); n > 0 {
		out.Cause = make([]modeljson.Exception, n)
		for i, cause := range e.Cause {
			if cause != nil {
				ExceptionModelJSON(cause, &out.Cause[i])
			}
		}
	}
	if n := len(e.Stacktrace); n > 0 {
		out.Stacktrace = make([]modeljson.StacktraceFrame, n)
		for i, frame := range e.Stacktrace {
			if frame != nil {
				StacktraceFrameModelJSON(frame, &out.Stacktrace[i])
			}
		}
	}
}
