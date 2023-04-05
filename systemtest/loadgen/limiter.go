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
	"time"

	"golang.org/x/time/rate"
)

// GetNewLimiter returns a rate limiter that is configured to rate limit burst per interval.
// eps is calculated as burst/interval(sec), and burst size is set as it is.
// When using it to send events at steady interval, the caller should control
// the events size to be equal to burst, and use `WaitN(ctx, burst)`.
// For example, if the event rate is 100/5s(burst=100,interval=5s) the eps is 20,
// Meaning to send 100 events, the limiter will wait for 5 seconds.
func GetNewLimiter(burst int, interval time.Duration) *rate.Limiter {
	if burst <= 0 || interval <= 0 {
		return rate.NewLimiter(rate.Inf, 0)
	}
	eps := float64(burst) / interval.Seconds()
	return rate.NewLimiter(rate.Limit(eps), burst)
}
