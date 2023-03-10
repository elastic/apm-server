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

package loadgen

import (
	"math"

	"golang.org/x/time/rate"
)

func GetNewLimiter(epm int, burst int) *rate.Limiter {
	if epm <= 0 {
		return rate.NewLimiter(rate.Inf, 0)
	}
	eps := float64(epm) / float64(60)
	return rate.NewLimiter(rate.Limit(eps), getBurstSize(int(math.Ceil(eps)), burst))
}

func getBurstSize(eps int, burst int) int {
	// Use default value when the burst is not set or invalid
	if burst <= 0 {
		burst := eps * 2
		// Allow for a batch to have 1000 events minimum
		if burst < 1000 {
			burst = 1000
		}
		return burst
	}

	return burst
}
