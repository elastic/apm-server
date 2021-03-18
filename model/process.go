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
	"github.com/elastic/beats/v7/libbeat/common"
)

type Process struct {
	Pid         int
	Ppid        *int
	Title       string
	Argv        []string
	CommandLine string
	Executable  string
}

func (p *Process) fields() common.MapStr {
	var proc mapStr
	if p.Pid != 0 {
		proc.set("pid", p.Pid)
	}
	if p.Ppid != nil {
		proc.set("ppid", *p.Ppid)
	}
	if len(p.Argv) > 0 {
		proc.set("args", p.Argv)
	}
	proc.maybeSetString("title", p.Title)
	proc.maybeSetString("command_line", p.CommandLine)
	proc.maybeSetString("executable", p.Executable)
	return common.MapStr(proc)
}
