// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package batchprocessor

import (
	"go.opentelemetry.io/collector/consumer/pdata"
)

// splitTraces removes spans from the input trace and returns a new trace of the specified size.
func splitTraces(size int, src pdata.Traces) pdata.Traces {
	if src.SpanCount() <= size {
		return src
	}
	totalCopiedSpans := 0
	dest := pdata.NewTraces()

	src.ResourceSpans().RemoveIf(func(srcRs pdata.ResourceSpans) bool {
		// If we are done skip everything else.
		if totalCopiedSpans == size {
			return false
		}

		destRs := dest.ResourceSpans().AppendEmpty()
		srcRs.Resource().CopyTo(destRs.Resource())

		srcRs.InstrumentationLibrarySpans().RemoveIf(func(srcIls pdata.InstrumentationLibrarySpans) bool {
			// If we are done skip everything else.
			if totalCopiedSpans == size {
				return false
			}

			destIls := destRs.InstrumentationLibrarySpans().AppendEmpty()
			srcIls.InstrumentationLibrary().CopyTo(destIls.InstrumentationLibrary())

			// If possible to move all metrics do that.
			srcSpansLen := srcIls.Spans().Len()
			if size-totalCopiedSpans >= srcSpansLen {
				totalCopiedSpans += srcSpansLen
				srcIls.Spans().MoveAndAppendTo(destIls.Spans())
				return true
			}

			srcIls.Spans().RemoveIf(func(srcSpan pdata.Span) bool {
				// If we are done skip everything else.
				if totalCopiedSpans == size {
					return false
				}
				srcSpan.CopyTo(destIls.Spans().AppendEmpty())
				totalCopiedSpans++
				return true
			})
			return false
		})
		return srcRs.InstrumentationLibrarySpans().Len() == 0
	})

	return dest
}
