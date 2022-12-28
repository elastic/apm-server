// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package interval

import (
	"fmt"
	"time"
)

// FormatDuration formats the duration in minutes (when the duration >= 1m), or
// seconds if smaller.
func FormatDuration(d time.Duration) string {
	if duration := d.Minutes(); duration >= 1 {
		return fmt.Sprintf("%.0fm", duration)
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}
