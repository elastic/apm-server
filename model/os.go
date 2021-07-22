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

// OS holds information about the operating system.
type OS struct {
	// Platform holds the operating system platform, e.g. centos, ubuntu, windows.
	Platform string

	// Full holds the full operating system name, including the version or code name.
	Full string

	// Type categorizes the operating system into one of the broad commercial families.
	//
	// If specified, Type must beone of the following (lowercase): linux, macos, unix, windows.
	// If the OS you’re dealing with is not in the list, the field should not be populated.
	Type string
}

func (o *OS) fields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("platform", o.Platform)
	fields.maybeSetString("full", o.Full)
	fields.maybeSetString("type", o.Type)
	return common.MapStr(fields)
}
