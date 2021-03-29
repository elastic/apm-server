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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/tests"
)

func TestProcessTransform(t *testing.T) {
	processTitle := "node"
	commandLine := "node run.js"
	executablePath := "/usr/bin/node"
	argv := []string{
		"node",
		"server.js",
	}

	tests := []struct {
		Process Process
		Output  common.MapStr
	}{
		{
			Process: Process{},
			Output:  nil,
		},
		{
			Process: Process{
				Pid:         123,
				Ppid:        tests.IntPtr(456),
				Title:       processTitle,
				Argv:        argv,
				CommandLine: commandLine,
				Executable:  executablePath,
			},
			Output: common.MapStr{
				"pid":          123,
				"ppid":         456,
				"title":        processTitle,
				"args":         argv,
				"command_line": commandLine,
				"executable":   executablePath,
			},
		},
	}

	for _, test := range tests {
		output := test.Process.fields()
		assert.Equal(t, test.Output, output)
	}
}
