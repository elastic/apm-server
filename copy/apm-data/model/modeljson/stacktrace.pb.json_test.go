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
	"testing"

	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestStacktraceToModelJSON(t *testing.T) {
	vars := randomKv(t)

	testCases := map[string]struct {
		proto    *modelpb.StacktraceFrame
		expected *modeljson.StacktraceFrame
	}{
		"empty": {
			proto:    &modelpb.StacktraceFrame{},
			expected: &modeljson.StacktraceFrame{},
		},
		"no pointers": {
			proto: &modelpb.StacktraceFrame{
				Filename:       "frame1_filename",
				Classname:      "frame1_classname",
				ContextLine:    "frame1_contextline",
				Module:         "frame1_module",
				Function:       "frame1_function",
				AbsPath:        "frame1_abspath",
				SourcemapError: "frame1_sourcemaperror",
				Original: &modelpb.Original{
					AbsPath:      "orig1_abspath",
					Filename:     "orig1_filename",
					Classname:    "orig1_classname",
					Function:     "orig1_function",
					LibraryFrame: true,
				},
				LibraryFrame:        true,
				SourcemapUpdated:    true,
				ExcludeFromGrouping: true,
			},
			expected: &modeljson.StacktraceFrame{
				Sourcemap: &modeljson.StacktraceFrameSourcemap{
					Error:   "frame1_sourcemaperror",
					Updated: true,
				},
				Line: &modeljson.StacktraceFrameLine{
					Context: "frame1_contextline",
				},
				Filename:  "frame1_filename",
				Classname: "frame1_classname",
				Module:    "frame1_module",
				Function:  "frame1_function",
				AbsPath:   "frame1_abspath",
				Original: &modeljson.StacktraceFrameOriginal{
					AbsPath:      "orig1_abspath",
					Filename:     "orig1_filename",
					Classname:    "orig1_classname",
					Function:     "orig1_function",
					LibraryFrame: true,
				},
				LibraryFrame:        true,
				ExcludeFromGrouping: true,
			},
		},
		"full": {
			proto: &modelpb.StacktraceFrame{
				Vars:           vars,
				Lineno:         uintPtr(1),
				Colno:          uintPtr(2),
				Filename:       "frame_filename",
				Classname:      "frame_classname",
				ContextLine:    "frame_contextline",
				Module:         "frame_module",
				Function:       "frame_function",
				AbsPath:        "frame_abspath",
				SourcemapError: "frame_sourcemaperror",
				Original: &modelpb.Original{
					AbsPath:      "orig_abspath",
					Filename:     "orig_filename",
					Classname:    "orig_classname",
					Lineno:       uintPtr(3),
					Colno:        uintPtr(4),
					Function:     "orig_function",
					LibraryFrame: true,
				},
				PreContext:          []string{"pre"},
				PostContext:         []string{"post"},
				LibraryFrame:        true,
				SourcemapUpdated:    true,
				ExcludeFromGrouping: true,
			},
			expected: &modeljson.StacktraceFrame{
				Sourcemap: &modeljson.StacktraceFrameSourcemap{
					Error:   "frame_sourcemaperror",
					Updated: true,
				},
				Vars: vars,
				Line: &modeljson.StacktraceFrameLine{
					Number:  uintPtr(1),
					Column:  uintPtr(2),
					Context: "frame_contextline",
				},
				Filename:  "frame_filename",
				Classname: "frame_classname",
				Module:    "frame_module",
				Function:  "frame_function",
				AbsPath:   "frame_abspath",
				Original: &modeljson.StacktraceFrameOriginal{
					AbsPath:      "orig_abspath",
					Filename:     "orig_filename",
					Classname:    "orig_classname",
					Lineno:       uintPtr(3),
					Colno:        uintPtr(4),
					Function:     "orig_function",
					LibraryFrame: true,
				},
				Context: &modeljson.StacktraceFrameContext{
					Pre:  []string{"pre"},
					Post: []string{"post"},
				},
				LibraryFrame:        true,
				ExcludeFromGrouping: true,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var out modeljson.StacktraceFrame
			StacktraceFrameModelJSON(tc.proto, &out)
			diff := cmp.Diff(*tc.expected, out, protocmp.Transform())
			require.Empty(t, diff)
		})
	}

}
