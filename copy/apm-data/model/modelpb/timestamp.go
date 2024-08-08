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

package modelpb

import "time"

// FromTime converts a time.Time to uint64 nanoseconds since Unix epoch.
func FromTime(t time.Time) uint64 {
	return uint64(t.UnixNano())
}

// ToTime converts uint64 nanoseconds since Unix epoch to a time.Time.
func ToTime(v uint64) time.Time {
	return time.Unix(int64(v/uint64(time.Second)), int64(v%uint64(time.Second))).UTC()
}
