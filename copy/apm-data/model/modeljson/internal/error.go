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

import "go.elastic.co/fastjson"

type Error struct {
	Exception   *Exception    `json:"exception,omitempty"`
	Log         *ErrorLog     `json:"log,omitempty"`
	ID          string        `json:"id,omitempty"`
	GroupingKey string        `json:"grouping_key,omitempty"`
	Culprit     string        `json:"culprit,omitempty"`
	Message     string        `json:"message,omitempty"`
	Type        string        `json:"type,omitempty"`
	StackTrace  string        `json:"stack_trace,omitempty"`
	Custom      KeyValueSlice `json:"custom,omitempty"`
}

type ErrorLog struct {
	Message      string            `json:"message,omitempty"`
	Level        string            `json:"level,omitempty"`
	ParamMessage string            `json:"param_message,omitempty"`
	LoggerName   string            `json:"logger_name,omitempty"`
	Stacktrace   []StacktraceFrame `json:"stacktrace,omitempty"`
}

type Exception struct {
	Message    string
	Module     string
	Code       string
	Attributes KeyValueSlice
	Stacktrace []StacktraceFrame
	Type       string
	Handled    *bool
	Cause      []Exception
}

func (e *Exception) MarshalFastJSON(w *fastjson.Writer) error {
	w.RawByte('[')
	if _, err := e.marshalOne(w, 0, 0); err != nil {
		return err
	}
	w.RawByte(']')
	return nil
}

func (e *Exception) marshalOne(w *fastjson.Writer, offset, parentOffset int) (int, error) {
	if offset > 0 {
		w.RawByte(',')
	}

	w.RawByte('{')
	firstAttr := true
	maybeComma := func() {
		if firstAttr {
			firstAttr = false
		} else {
			w.RawByte(',')
		}
	}
	if e.Message != "" {
		w.RawString(`"message":`)
		w.String(e.Message)
		firstAttr = false
	}
	if e.Type != "" {
		maybeComma()
		w.RawString(`"type":`)
		w.String(e.Type)
	}
	if e.Module != "" {
		maybeComma()
		w.RawString(`"module":`)
		w.String(e.Module)
	}
	if e.Code != "" {
		maybeComma()
		w.RawString(`"code":`)
		w.String(e.Code)
	}
	if e.Handled != nil {
		maybeComma()
		w.RawString(`"handled":`)
		w.Bool(*e.Handled)
	}
	if e.Attributes != nil {
		maybeComma()
		w.RawString(`"attributes":`)
		if err := e.Attributes.MarshalFastJSON(w); err != nil {
			return -1, err
		}
	}
	if offset > parentOffset+1 {
		// The parent of an exception in the resulting slice is at the offset
		// indicated by the `parent` field (0 index based), or the preceding
		// exception in the slice if the `parent` field is not set.
		maybeComma()
		w.RawString(`"parent":`)
		w.Int64(int64(parentOffset))
	}
	if len(e.Stacktrace) != 0 {
		maybeComma()
		w.RawString(`"stacktrace":`)
		w.RawByte('[')
		for i, v := range e.Stacktrace {
			if i != 0 {
				w.RawByte(',')
			}
			if err := fastjson.Marshal(w, &v); err != nil {
				return -1, err
			}
		}
		w.RawByte(']')
	}
	w.RawByte('}')

	nextOffset := offset + 1
	for _, cause := range e.Cause {
		var err error
		nextOffset, err = cause.marshalOne(w, nextOffset, offset)
		if err != nil {
			return -1, err
		}
	}
	return nextOffset, nil
}
