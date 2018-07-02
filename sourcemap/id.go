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
	"fmt"
	"strings"
)

type Id struct {
	ServiceName    string
	ServiceVersion string
	Path           string
}

func (i *Id) Key() string {
	info := []string{}
	info = add(info, i.ServiceName)
	info = add(info, i.ServiceVersion)
	info = add(info, i.Path)
	return strings.Join(info, "_")
}

func (i *Id) String() string {
	return fmt.Sprintf("Service Name: '%s', Service Version: '%s' and Path: '%s'",
		i.ServiceName,
		i.ServiceVersion,
		i.Path)
}

func (i *Id) Valid() bool {
	return i.ServiceName != "" && i.ServiceVersion != "" && i.Path != ""
}

func add(a []string, s string) []string {
	if s != "" {
		a = append(a, s)
	}
	return a
}
