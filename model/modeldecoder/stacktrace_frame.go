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

package modeldecoder

import (
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/field"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/utility"
)

var (
	errInvalidStacktraceFrameType = errors.New("invalid type for stacktrace frame")
)

func DecodeStacktraceFrame(input interface{}, hasShortFieldNames bool, err error) (*model.StacktraceFrame, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errInvalidStacktraceFrameType
	}
	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(hasShortFieldNames)
	frame := model.StacktraceFrame{
		AbsPath:      decoder.StringPtr(raw, fieldName("abs_path")),
		Filename:     decoder.StringPtr(raw, fieldName("filename")),
		Classname:    decoder.StringPtr(raw, fieldName("classname")),
		Lineno:       decoder.IntPtr(raw, fieldName("lineno")),
		Colno:        decoder.IntPtr(raw, fieldName("colno")),
		ContextLine:  decoder.StringPtr(raw, fieldName("context_line")),
		Module:       decoder.StringPtr(raw, fieldName("module")),
		Function:     decoder.StringPtr(raw, fieldName("function")),
		LibraryFrame: decoder.BoolPtr(raw, "library_frame"),
		Vars:         decoder.MapStr(raw, "vars"),
		PreContext:   decoder.StringArr(raw, fieldName("pre_context")),
		PostContext:  decoder.StringArr(raw, fieldName("post_context")),
	}
	return &frame, decoder.Err
}
