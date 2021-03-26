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

package elasticsearch

import (
	"time"

	"github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
)

type backoffFunc func(int) time.Duration

var (
	// DefaultBackoffConfig is the default backoff configuration used for
	// es clients.
	DefaultBackoffConfig = elasticsearch.Backoff{
		Init: time.Second,
		Max:  time.Minute,
	}

	// DefaultBackoff is an exponential backoff configured with default
	// backoff settings.
	DefaultBackoff = exponentialBackoff(DefaultBackoffConfig)
)

func exponentialBackoff(b elasticsearch.Backoff) backoffFunc {
	return func(attempts int) time.Duration {
		// Attempts starts at 1, after there's already been a failure.
		// https://github.com/elastic/go-elasticsearch/blob/de2391/estransport/estransport.go#L339
		next := b.Init * (1 << (attempts - 1))
		if next > b.Max {
			next = b.Max
		}
		return next
	}
}
