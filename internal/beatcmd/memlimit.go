package beatcmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/KimMachineGun/automemlimit/memlimit"
)

func adjustMemlimit(d time.Duration, logger *slog.Logger) error {
	if _, err := memlimit.SetGoMemLimitWithOpts(
		memlimit.WithLogger(logger),
		memlimit.WithRefreshInterval(d),
		memlimit.WithRatio(0.9),
	); err != nil {
		return fmt.Errorf("failed to set go memlimit: %w", err)
	}
	return nil
}
