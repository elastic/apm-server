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

package sourcemap

import (
	"context"
	"sync"
	"time"

	"github.com/elastic/beats/v7/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
)

// BatchProcessor is a model.BatchProcessor that performs source mapping for
// span and error events. Any errors fetching source maps, including the
// timeout expiring, will result in the StacktraceFrame.SourcemapError field
// being set; the error will not be returned.
type BatchProcessor struct {
	// Fetcher is the Fetcher to use for fetching source maps.
	Fetcher Fetcher

	// Timeout holds a timeout for each ProcessBatch call, to limit how
	// much time is spent fetching source maps.
	//
	// If Timeout is <= 0, it will be ignored.
	Timeout time.Duration
}

// ProcessBatch processes spans and errors, applying source maps
// to their stack traces.
func (p BatchProcessor) ProcessBatch(ctx context.Context, batch *model.Batch) error {
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}
	for _, event := range *batch {
		if event.Service.Name == "" || event.Service.Version == "" {
			continue
		}
		switch {
		case event.Span != nil:
			p.processStacktraceFrames(ctx, &event.Service, event.Span.Stacktrace...)
		case event.Error != nil:
			if event.Error.Log != nil {
				p.processStacktraceFrames(ctx, &event.Service, event.Error.Log.Stacktrace...)
			}
			if event.Error.Exception != nil {
				p.processException(ctx, &event.Service, event.Error.Exception)
			}
		}
	}
	return nil
}

func (p BatchProcessor) processException(ctx context.Context, service *model.Service, exception *model.Exception) {
	p.processStacktraceFrames(ctx, service, exception.Stacktrace...)
	for _, cause := range exception.Cause {
		p.processException(ctx, service, &cause)
	}
}

// source map algorithm:
//
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
// - sourcemap.updated is set to true
func (p BatchProcessor) processStacktraceFrames(ctx context.Context, service *model.Service, frames ...*model.StacktraceFrame) error {
	prevFunction := "<anonymous>"
	for i := len(frames) - 1; i >= 0; i-- {
		frame := frames[i]
		if mapped, function := p.processStacktraceFrame(ctx, service, frame, prevFunction); mapped {
			prevFunction = function
		}
	}
	return nil
}

func (p BatchProcessor) processStacktraceFrame(
	ctx context.Context,
	service *model.Service,
	frame *model.StacktraceFrame,
	prevFunction string,
) (bool, string) {
	if frame.Colno == nil || frame.Lineno == nil || frame.AbsPath == "" {
		return false, ""
	}

	path := utility.CleanUrlPath(frame.AbsPath)
	mapper, err := p.Fetcher.Fetch(ctx, service.Name, service.Version, path)
	if err != nil {
		frame.SourcemapError = err.Error()
		getProcessorLogger().Debugf("failed to fetch source map: %s", frame.SourcemapError)
		return false, ""
	}
	if mapper == nil {
		return false, ""
	}
	file, function, lineno, colno, ctxLine, preCtx, postCtx, ok := Map(mapper, *frame.Lineno, *frame.Colno)
	if !ok {
		return false, ""
	}

	// Store original source information.
	frame.Original.Colno = frame.Colno
	frame.Original.AbsPath = frame.AbsPath
	frame.Original.Function = frame.Function
	frame.Original.Lineno = frame.Lineno
	frame.Original.Filename = frame.Filename
	frame.Original.Classname = frame.Classname

	if file != "" {
		frame.Filename = file
	}
	frame.Colno = &colno
	frame.Lineno = &lineno
	frame.AbsPath = path
	frame.SourcemapUpdated = true
	frame.Function = prevFunction
	frame.ContextLine = ctxLine
	frame.PreContext = preCtx
	frame.PostContext = postCtx
	if function == "" {
		function = "<unknown>"
	}
	return true, function
}

func getProcessorLogger() *logp.Logger {
	processorLoggerOnce.Do(func() {
		// We use a rate limited logger to avoid spamming the logs
		// due to issues communicating with Elasticsearch, for example.
		processorLogger = logp.NewLogger(
			logs.Stacktrace,
			logs.WithRateLimit(time.Minute),
		)
	})
	return processorLogger
}

var (
	processorLoggerOnce sync.Once
	processorLogger     *logp.Logger
)
