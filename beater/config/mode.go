package config

import "strings"

//Mode enumerates the APM Server env
type Mode uint8

const (
	// ModeProduction is the default mode
	ModeProduction Mode = iota

	// ModeExperimental should only be used in development environments. It allows to circumvent some restrictions
	// on the Intake API for faster development.
	ModeExperimental
)

// Unpack parses the given string into a Mode value
func (m *Mode) Unpack(s string) error {
	if strings.ToLower(s) == "experimental" {
		*m = ModeExperimental
		return nil
	}
	*m = ModeProduction
	return nil
}
