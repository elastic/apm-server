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

package logs_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func TestWithRateLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logger, observed := logptest.NewTestingLoggerWithObserver(t, "bo")

		const interval = 100 * time.Millisecond
		limitedLogger := logger.WithOptions(logs.WithRateLimit(interval))

		// Log twice in quick succession; the 2nd call will be ignored due to rate-limiting.
		limitedLogger.Info("hello")
		limitedLogger.Info("hello")
		assert.Equal(t, 1, observed.Len())

		// Sleep until the configured interval has elapsed, which should allow another
		// record to be logged.
		time.Sleep(interval * 100)
		limitedLogger.Info("hello")
		assert.Equal(t, 2, observed.Len())
	})
}
