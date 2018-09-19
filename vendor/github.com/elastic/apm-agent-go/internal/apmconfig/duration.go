package apmconfig

import (
	"time"
)

// ParseDuration parses value as a duration, appending defaultSuffix if value
// does not have one.
func ParseDuration(value, defaultSuffix string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil && defaultSuffix != "" {
		// We allow the value to have no suffix, in which case we append
		// defaultSuffix ("s" for flush interval) for compatibility with
		// configuration for other Elastic APM agents.
		var err2 error
		d, err2 = time.ParseDuration(value + defaultSuffix)
		if err2 == nil {
			err = nil
		}
	}
	if err != nil {
		return 0, err
	}
	return d, nil
}
