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
	"github.com/elastic/apm-server/model/metadata"
)

func decodeProcess(input map[string]interface{}, out *metadata.Process) {
	if input == nil {
		return
	}

	decodeString(input, "title", &out.Title)
	decodeInt(input, "pid", &out.Pid)

	var ppid int
	if decodeInt(input, "ppid", &ppid) {
		// TODO(axw) consider using a negative value as a sentinel
		// value for unset ppid, since pids cannot be negative.
		out.Ppid = &ppid
	}

	if argv, ok := input["argv"].([]interface{}); ok {
		out.Argv = out.Argv[:0]
		for _, arg := range argv {
			if strval, ok := arg.(string); ok {
				out.Argv = append(out.Argv, strval)
			}
		}
	}
}
