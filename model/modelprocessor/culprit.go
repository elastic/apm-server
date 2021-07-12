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

	"github.com/elastic/apm-server/model"
)

// SetCulprit is a model.BatchProcessor that sets or updates the culprit for RUM
// errors, after source mapping and identifying library frames.
type SetCulprit struct{}

// ProcessBatch sets or updates the culprit for RUM errors.
func (s SetCulprit) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for _, event := range *b {
		if event.Error != nil {
			s.processError(ctx, event.Error)
		}
	}
	return nil
}

func (s SetCulprit) processError(ctx context.Context, event *model.Error) {
	var culpritFrame *model.StacktraceFrame
	if event.Log != nil {
		culpritFrame = s.findSourceMappedNonLibraryFrame(event.Log.Stacktrace)
	}
	if culpritFrame == nil && event.Exception != nil {
		culpritFrame = s.findSourceMappedNonLibraryFrame(event.Exception.Stacktrace)
	}
	if culpritFrame == nil {
		return
	}
	culprit := culpritFrame.Filename
	if culprit == "" {
		culprit = culpritFrame.Classname
	}
	if culpritFrame.Function != "" {
		culprit += " in " + culpritFrame.Function
	}
	event.Culprit = culprit
}

func (s SetCulprit) findSourceMappedNonLibraryFrame(frames []*model.StacktraceFrame) *model.StacktraceFrame {
	for _, frame := range frames {
		if frame.SourcemapUpdated && !frame.LibraryFrame {
			return frame
		}
	}
	return nil
}
