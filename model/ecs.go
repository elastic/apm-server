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
	"strconv"
	"strings"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

func CopyECS(fields common.MapStr) {
	// context.tags -> labels
	utility.Add(fields, "labels", fields["context"].(common.MapStr)["tags"])

	// context.request.url.protocol -> url.scheme (minus trailing colon)
	if scheme, err := fields.GetValue("context.request.url.protocol"); err == nil {
		if schemeStr, ok := scheme.(string); ok {
			utility.MergeAdd(fields, "url", common.MapStr{"scheme": strings.TrimSuffix(schemeStr, ":")})
		}
	}

	// context.request.url.port -> url.port (as int)
	if port, err := fields.GetValue("context.request.url.port"); err == nil {
		if portStr, ok := port.(string); ok {
			if portNum, err := strconv.Atoi(portStr); err == nil {
				utility.MergeAdd(fields, "url", common.MapStr{"port": portNum})
			}
		}
	}
}
