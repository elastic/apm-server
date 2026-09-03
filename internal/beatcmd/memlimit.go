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
	"log/slog"
	"time"

	"github.com/KimMachineGun/automemlimit/memlimit"
)

// adjustMemlimit sets Go's soft memory limit (GOMEMLIMIT) to 90% of the
// process's cgroup or system memory limit, refreshing every d until ctx is
// canceled.
func adjustMemlimit(ctx context.Context, d time.Duration, logger *slog.Logger) error {
	if _, err := memlimit.Set(
		memlimit.WithProvider(
			memlimit.ApplyFallback(
				memlimit.FromCgroup,
				memlimit.FromSystem,
			),
		),
		memlimit.WithLogger(logger),
		memlimit.WithRefreshInterval(ctx, d),
		memlimit.WithRatio(0.9),
	); err != nil {
		// automemlimit already logs this via the configured logger, and a
		// failed adjustment is non-fatal, so don't re-log or return early:
		// the refresh loop stays running and may recover on a later tick.
		_ = err
	}

	<-ctx.Done()
	return ctx.Err()
}
