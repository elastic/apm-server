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

// UserExperience holds real user (browser) experience metrics.
type UserExperience struct {
	// CumulativeLayoutShift holds the Cumulative Layout Shift (CLS) metric value,
	// or a negative value if CLS is unknown. See https://web.dev/cls/
	CumulativeLayoutShift float64

	// FirstInputDelay holds the First Input Delay (FID) metric value,
	// or a negative value if FID is unknown. See https://web.dev/fid/
	FirstInputDelay float64

	// TotalBlockingTime holds the Total Blocking Time (TBT) metric value,
	// or a negative value if TBT is unknown. See https://web.dev/tbt/
	TotalBlockingTime float64
}

func (u *UserExperience) Fields() common.MapStr {
	if u == nil {
		return nil
	}
	var fields mapStr
	if u.CumulativeLayoutShift >= 0 {
		fields.set("cls", u.CumulativeLayoutShift)
	}
	if u.FirstInputDelay >= 0 {
		fields.set("fid", u.FirstInputDelay)
	}
	if u.TotalBlockingTime >= 0 {
		fields.set("tbt", u.TotalBlockingTime)
	}
	return common.MapStr(fields)
}
