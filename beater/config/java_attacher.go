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

package config

// JavaAttacherConfig holds configuration information for running a java
// attacher jarfile.
type JavaAttacherConfig struct {
	Enabled    bool `config:"enabled"`
	Continuous bool `config:"continuous"`

	IncludeAll bool     `config:"include_all"`
	IncludePID []string `config:"include_pid"`

	IncludeMain  []string `config:"include_main"`
	IncludeVMArg []string `config:"include_vmarg"`
	IncludeUser  []string `config:"include_user"`

	ExcludeMain  []string `config:"exclude_main"`
	ExcludeVMArg []string `config:"exclude_vmarg"`
	ExcludeUser  []string `config:"exclude_user"`

	Config []string `config:"config"`
}

// The attacher version should be fixed for a given version of apm-server
// https://github.com/elastic/apm-server/issues/4830#issuecomment-863207642
var javaAttacherVersion = ""

func defaultJavaAttacherConfig() JavaAttacherConfig {
	return JavaAttacherConfig{Enabled: false}
}
