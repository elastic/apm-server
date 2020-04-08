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
	"context"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model/metadata"

	"github.com/elastic/apm-server/transform"
)

var (
	msgServiceInvalidForSourcemapping = "Cannot apply sourcemap without a service name or service version"
)

type Stacktrace []*StacktraceFrame

func (st *Stacktrace) Transform(ctx context.Context, tctx *transform.Context, service *metadata.Service) []common.MapStr {
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

	if tctx.Config.SourcemapStore == nil {
		return st.transform(tctx, noSourcemapping)
	}
	if service == nil || service.Name == "" || service.Version == "" {
		logp.NewLogger(logs.Stacktrace).Warn(msgServiceInvalidForSourcemapping)
		return st.transform(tctx, noSourcemapping)
	}

	var errMsg string
	var sourcemapErrorSet = map[string]interface{}{}
	logger := logp.NewLogger(logs.Stacktrace)
	fct := "<anonymous>"
	return st.transform(tctx, func(frame *StacktraceFrame) {
		fct, errMsg = frame.applySourcemap(ctx, tctx.Config.SourcemapStore, service, fct)
		if errMsg != "" {
			if _, ok := sourcemapErrorSet[errMsg]; !ok {
				logger.Warn(errMsg)
				sourcemapErrorSet[errMsg] = nil
			}
		}
	})
}

func (st *Stacktrace) transform(ctx *transform.Context, apply func(*StacktraceFrame)) []common.MapStr {
	frameCount := len(*st)
	if frameCount == 0 {
		return nil
	}

	var fr *StacktraceFrame
	frames := make([]common.MapStr, frameCount)
	for idx := frameCount - 1; idx >= 0; idx-- {
		fr = (*st)[idx]
		apply(fr)
		frames[idx] = fr.Transform(ctx)
	}
	return frames
}

func noSourcemapping(_ *StacktraceFrame) {}
