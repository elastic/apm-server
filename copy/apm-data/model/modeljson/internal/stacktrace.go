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

type StacktraceFrame struct {
	Sourcemap           *StacktraceFrameSourcemap `json:"sourcemap,omitempty"`
	Original            *StacktraceFrameOriginal  `json:"original,omitempty"`
	Context             *StacktraceFrameContext   `json:"context,omitempty"`
	Line                *StacktraceFrameLine      `json:"line,omitempty"`
	Filename            string                    `json:"filename,omitempty"`
	Classname           string                    `json:"classname,omitempty"`
	Module              string                    `json:"module,omitempty"`
	Function            string                    `json:"function,omitempty"`
	AbsPath             string                    `json:"abs_path,omitempty"`
	Vars                KeyValueSlice             `json:"vars,omitempty"`
	LibraryFrame        bool                      `json:"library_frame,omitempty"`
	ExcludeFromGrouping bool                      `json:"exclude_from_grouping"`
}

type StacktraceFrameContext struct {
	Pre  []string `json:"pre,omitempty"`
	Post []string `json:"post,omitempty"`
}

type StacktraceFrameLine struct {
	Number  *uint32 `json:"number,omitempty"`
	Column  *uint32 `json:"column,omitempty"`
	Context string  `json:"context,omitempty"`
}

type StacktraceFrameSourcemap struct {
	Error   string `json:"error,omitempty"`
	Updated bool   `json:"updated,omitempty"`
}

type StacktraceFrameOriginal struct {
	AbsPath      string  `json:"abs_path,omitempty"`
	Filename     string  `json:"filename,omitempty"`
	Classname    string  `json:"classname,omitempty"`
	Lineno       *uint32 `json:"lineno,omitempty"`
	Colno        *uint32 `json:"colno,omitempty"`
	Function     string  `json:"function,omitempty"`
	LibraryFrame bool    `json:"library_frame,omitempty"`
}
