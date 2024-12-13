package terraform

import (
	"testing"
)

// tfLoggerv2 wraps a testing.TB to implement the printfer interface
// required by terraform-exec logger.
type tfLoggerv2 struct {
	testing.TB
}

// Printf implements terraform-exec.printfer interface
func (l *tfLoggerv2) Printf(format string, v ...interface{}) {
	l.Logf(format, v...)
}
