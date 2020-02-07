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
	"errors"
	"regexp"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/sourcemap"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
)

var (
	errInvalidStacktraceType          = errors.New("invalid type for stacktrace")
	msgServiceInvalidForSourcemapping = "Cannot apply sourcemap without a service name or service version"
)

type Stacktrace []*StacktraceFrame

func DecodeStacktrace(input interface{}, err error) (*Stacktrace, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.([]interface{})
	if !ok {
		return nil, errInvalidStacktraceType
	}
	st := make(Stacktrace, len(raw))
	for idx, fr := range raw {
		st[idx], err = DecodeStacktraceFrame(fr, err)
	}
	return &st, err
}

func (st *Stacktrace) Transform(libraryPattern, excludeFromGrouping *regexp.Regexp, sourcemapStore *sourcemap.Store, service *metadata.Service) []common.MapStr {
	if st == nil {
		return nil
	}
	// source map algorithm:
	// apply source mapping frame by frame
	// if no source map could be found, set updated to false and set sourcemap error
	// otherwise use source map library for mapping and update
	// - filename: only if it was found
	// - function:
	//   * should be moved down one stack trace frame,
	//   * the function name of the first frame is set to <anonymous>
	//   * if one frame is not found in the source map, this frame is left out and
	//   the function name from the previous frame is used
	//   * if a mapping could be applied but no function name is found, the
	//   function name for the next frame is set to <unknown>
	// - colno
	// - lineno
	// - abs_path is set to the cleaned abs_path
	// - sourcmeap.updated is set to true

	if sourcemapStore == nil {
		return st.transform(libraryPattern, excludeFromGrouping, noSourcemapping)
	}
	// service comes from metadata, not from the event
	if service == nil || service.Name == nil || service.Version == nil {
		logp.NewLogger(logs.Stacktrace).Warn(msgServiceInvalidForSourcemapping)
		return st.transform(libraryPattern, excludeFromGrouping, noSourcemapping)
	}

	var errMsg string
	var sourcemapErrorSet = map[string]interface{}{}
	logger := logp.NewLogger(logs.Stacktrace)
	fct := "<anonymous>"
	return st.transform(libraryPattern, excludeFromGrouping, func(frame *StacktraceFrame) {
		fct, errMsg = frame.applySourcemap(sourcemapStore, service, fct)
		if errMsg != "" {
			if _, ok := sourcemapErrorSet[errMsg]; !ok {
				logger.Warn(errMsg)
				sourcemapErrorSet[errMsg] = nil
			}
		}
	})
}

func (st *Stacktrace) transform(libraryPattern, excludeFromGrouping *regexp.Regexp, apply func(*StacktraceFrame)) []common.MapStr {
	frameCount := len(*st)
	if frameCount == 0 {
		return nil
	}

	var fr *StacktraceFrame
	frames := make([]common.MapStr, frameCount)
	for idx := frameCount - 1; idx >= 0; idx-- {
		fr = (*st)[idx]
		apply(fr)
		frames[idx] = fr.transform(libraryPattern, excludeFromGrouping)
	}
	return frames
}

func noSourcemapping(_ *StacktraceFrame) {}
