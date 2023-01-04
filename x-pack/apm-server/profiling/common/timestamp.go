// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package common

import (
	"time"
)

// GetStartOfWeekFromTime returns the seconds since the epoch for the given time's week.
// Updating an existing index entry is expensive as the old entry is marked 'deleted'
// and a new entry is created. But this only happens if at least one value changed.
// Since a per-week resolution is enough for us, we remove the second-of-the-day.
func GetStartOfWeekFromTime(t time.Time) uint32 {
	return uint32(t.Truncate(time.Hour * 24 * 7).Unix())
}
