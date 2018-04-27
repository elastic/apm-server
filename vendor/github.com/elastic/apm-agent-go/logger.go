package elasticapm

// Logger is an interface for logging, used by the tracer
// to log tracer errors and other interesting events.
type Logger interface {
	// Debugf logs a message at debug level.
	Debugf(format string, args ...interface{})

	// Errorf logs a message at error level.
	Errorf(format string, args ...interface{})
}
