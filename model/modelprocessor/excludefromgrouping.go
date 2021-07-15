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

package modelprocessor

import (
	"context"
	"regexp"

	"github.com/elastic/apm-server/model"
)

// SetExcludeFromGrouping is a model.BatchProcessor that identifies stack frames
// to exclude from error grouping for RUM, using a configurable regular expression.
type SetExcludeFromGrouping struct {
	Pattern *regexp.Regexp
}

// ProcessBatch processes the stack traces of spans and errors in b, updating
// the exclude_from_grouping  for stack frames based on whether they have a filename
// matching the regular expression.
func (s SetExcludeFromGrouping) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for _, event := range *b {
		switch {
		case event.Span != nil:
			s.processSpan(ctx, event.Span)
		case event.Error != nil:
			s.processError(ctx, event.Error)
		}
	}
	return nil
}

func (s SetExcludeFromGrouping) processSpan(ctx context.Context, event *model.Span) {
	s.processStacktraceFrames(ctx, event.Stacktrace...)
}

func (s SetExcludeFromGrouping) processError(ctx context.Context, event *model.Error) {
	if event.Log != nil {
		s.processStacktraceFrames(ctx, event.Log.Stacktrace...)
	}
	if event.Exception != nil {
		s.processException(ctx, event.Exception)
	}
}

func (s SetExcludeFromGrouping) processException(ctx context.Context, exception *model.Exception) {
	s.processStacktraceFrames(ctx, exception.Stacktrace...)
	for _, cause := range exception.Cause {
		s.processException(ctx, &cause)
	}
}

func (s SetExcludeFromGrouping) processStacktraceFrames(ctx context.Context, frames ...*model.StacktraceFrame) {
	for _, frame := range frames {
		frame.ExcludeFromGrouping = frame.Filename != "" && s.Pattern.MatchString(frame.Filename)
	}
}
