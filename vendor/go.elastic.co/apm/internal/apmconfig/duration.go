package apmconfig

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ParseDuration parses s as a duration, accepting a subset
// of the syntax supported by time.ParseDuration.
//
// Valid time units are "ms", "s", "m".
func ParseDuration(s string) (time.Duration, error) {
	orig := s
	var mul time.Duration = 1
	if strings.HasPrefix(s, "-") {
		mul = -1
		s = s[1:]
	}

	sep := -1
	for i, c := range s {
		if sep == -1 {
			if c < '0' || c > '9' {
				sep = i
				break
			}
		}
	}
	if sep == -1 {
		return 0, fmt.Errorf("missing unit in duration %s (allowed units: ms, s, m)", orig)
	}

	n, err := strconv.ParseInt(s[:sep], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %s", orig)
	}
	switch s[sep:] {
	case "ms":
		mul *= time.Millisecond
	case "s":
		mul *= time.Second
	case "m":
		mul *= time.Minute
	default:
		for _, c := range s[sep:] {
			if unicode.IsSpace(c) {
				return 0, fmt.Errorf("invalid character %q in duration %s", c, orig)
			}
		}
		return 0, fmt.Errorf("invalid unit in duration %s (allowed units: ms, s, m)", orig)
	}
	return mul * time.Duration(n), nil
}
