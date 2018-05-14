package utility

import (
	"strconv"
	"strings"
)

// ForwardedHeader holds information extracted from a "Forwarded" HTTP header.
type ForwardedHeader struct {
	For   string
	Host  string
	Proto string
}

// ParseForwarded parses a "Forwarded" HTTP header.
func ParseForwarded(f string) ForwardedHeader {
	// We only consider the first value in the sequence,
	// if there are multiple. Disregard everything after
	// the first comma.
	if comma := strings.IndexRune(f, ','); comma != -1 {
		f = f[:comma]
	}
	var result ForwardedHeader
	for f != "" {
		field := f
		if semi := strings.IndexRune(f, ';'); semi != -1 {
			field = f[:semi]
			f = f[semi+1:]
		} else {
			f = ""
		}
		eq := strings.IndexRune(field, '=')
		if eq == -1 {
			// Malformed field, ignore.
			continue
		}
		key := strings.TrimSpace(field[:eq])
		value := strings.TrimSpace(field[eq+1:])
		if len(value) > 0 && value[0] == '"' {
			var err error
			value, err = strconv.Unquote(value)
			if err != nil {
				// Malformed, ignore
				continue
			}
		}
		switch strings.ToLower(key) {
		case "for":
			result.For = value
		case "host":
			result.Host = value
		case "proto":
			result.Proto = value
		}
	}
	return result
}
