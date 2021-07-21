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

// Page consists of URL and referer
type Page struct {
	URL     *URL
	Referer string
}

// Fields returns common.MapStr holding transformed data for attribute page.
func (page *Page) Fields() common.MapStr {
	if page == nil {
		return nil
	}
	var fields mapStr
	if page.URL != nil {
		// Remove in 8.0
		fields.set("url", page.URL.Original)
	}
	fields.maybeSetString("referer", page.Referer)
	return common.MapStr(fields)
}

// customFields transforms in, returning a copy with sanitized keys
// and normalized field values, suitable for storing as "custom"
// in transaction and error documents..
func customFields(in common.MapStr) common.MapStr {
	if len(in) == 0 {
		return nil
	}
	out := make(common.MapStr, len(in))
	for k, v := range in {
		out[sanitizeLabelKey(k)] = normalizeLabelValue(v)
	}
	return out
}
