package beatcmd

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/automaxprocs/maxprocs"

	"github.com/elastic/elastic-agent-libs/logp"
)

type logf func(string, ...interface{})

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
func adjustMaxProcs(ctx context.Context, d time.Duration, infof, errorf logf) error {
	setMaxProcs := func() {
		if _, err := maxprocs.Set(maxprocs.Logger(infof)); err != nil {
			errorf("failed to set GOMAXPROCS: %v", err)
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

func diffInfof(logger *logp.Logger) logf {
	var last string
	return func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		if msg != last {
			logger.Info(msg)
			last = msg
		}
	}
}
