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

package beatcmd

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/automaxprocs/maxprocs"

	"github.com/elastic/elastic-agent-libs/logp"
)

// adjustMaxProcs uses `maxprocs` to change the GOMAXPROCS respecting any
// CFS quotas, if set.
//
// This is necessary since the Go runtime will default to the number of CPUs
// available in the  machine it's running in, however, when running in a
// container or in a cgroup with resource limits, the disparity can be extreme.
//
// Having a significantly greater GOMAXPROCS set than the granted CFS quota
// results in a significant amount of time spent "throttling", essentially
// pausing the the running OS threads for the throttled period.
// Since the quotas may be updated without restarting the process, the
// GOMAXPROCS are adjusted every 30s.
func adjustMaxProcs(ctx context.Context, d time.Duration, logger *logp.Logger) error {
	infof := diffInfof(logger)
	setMaxProcs := func() {
		if _, err := maxprocs.Set(maxprocs.Logger(infof)); err != nil {
			logger.Errorf("failed to set GOMAXPROCS: %v", err)
		}
	}
	// set the gomaxprocs immediately.
	setMaxProcs()
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			setMaxProcs()
		}
	}
}

func diffInfof(logger *logp.Logger) func(string, ...interface{}) {
	var last string
	return func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		if msg != last {
			logger.Info(msg)
			last = msg
		}
	}
}
