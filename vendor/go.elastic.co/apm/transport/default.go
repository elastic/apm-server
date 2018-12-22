package transport

var (
	// Default is the default Transport, using the
	// ELASTIC_APM_* environment variables.
	//
	// If ELASTIC_APM_SERVER_URL is set to an invalid
	// location, Default will be set to a Transport
	// returning an error for every operation.
	Default Transport

	// Discard is a Transport on which all operations
	// succeed without doing anything.
	Discard = discardTransport{}
)

func init() {
	_, _ = InitDefault()
}

// InitDefault (re-)initializes Default, the default Transport, returning
// its new value along with the error that will be returned by the Transport
// if the environment variable configuration is invalid. The result is always
// non-nil.
func InitDefault() (Transport, error) {
	t, err := getDefault()
	Default = t
	return t, err
}

func getDefault() (Transport, error) {
	s, err := NewHTTPTransport()
	if err != nil {
		return discardTransport{err}, err
	}
	return s, nil
}
