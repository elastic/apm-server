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

type UserAgent struct {
	// Original holds the original, full, User-Agent string.
	Original string

	// Name holds the user_agent.name value from the parsed User-Agent string.
	// If Original is set, then this should typically not be set, as the full
	// User-Agent string can be parsed by ingest node.
	Name string
}

func (u *UserAgent) fields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("original", u.Original)
	fields.maybeSetString("name", u.Name)
	return common.MapStr(fields)
}
