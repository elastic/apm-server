package apmconfig

import (
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

// ParseDurationEnv gets the value of the environment variable envKey
// and, if set, parses it as a duration. If the value has no suffix,
// defaultSuffix is appended. If the environment variable is unset,
// defaultDuration is returned.
func ParseDurationEnv(envKey, defaultSuffix string, defaultDuration time.Duration) (time.Duration, error) {
	value := os.Getenv(envKey)
	if value == "" {
		return defaultDuration, nil
	}
	d, err := ParseDuration(value, defaultSuffix)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse %s", envKey)
	}
	return d, nil
}

// ParseBoolEnv gets the value of the environment variable envKey
// and, if set, parses it as a boolean. If the environment variable
// is unset, defaultValue is returned.
func ParseBoolEnv(envKey string, defaultValue bool) (bool, error) {
	value := os.Getenv(envKey)
	if value == "" {
		return defaultValue, nil
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse %s", envKey)
	}
	return b, nil
}
