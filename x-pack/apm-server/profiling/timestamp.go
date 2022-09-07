// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package profiling

import (
	"time"
)

// GetStartOfWeek returns the seconds since the epoch for time 00:00:00 of the current week.
// Updating an existing index entry is expensive as the old entry is marked 'deleted'
// and a new entry is created. But this only happens if at least one value changed.
// Since a per-week resolution is enough for us, we remove the second-of-the-day.
func GetStartOfWeek() uint32 {
	return GetStartOfWeekFromTime(time.Now())
}

// GetStartOfWeekFromTime is the same as GetStartOfWeek, except that it doesn't take the
// current date, but instead it takes the given date.
func GetStartOfWeekFromTime(t time.Time) uint32 {
	return uint32(t.Truncate(time.Hour * 24 * 7).Unix())
}
