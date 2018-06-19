package transport

import (
	"os"

	"github.com/elastic/apm-agent-go/internal/apmdebug"
)

var (
	// Default is the default Transport, using the
	// ELASTIC_APM_* environment variables.
	//
	// If ELASTIC_APM_SERVER_URL is not defined, then
	// Defaultwill be set to Discard. If it is defined,
	// but invalid, then Default will be set to a transport
	// returning an error for every operation.
	Default Transport

	// Discard is a Transport on which all operations
	// succeed without doing anything.
	Discard = discardTransport{}
)

func init() {
	_, _ = InitDefault()
}

// InitDefault (re-)initializes Default, the default transport, returning
// its new value along with the error that will be returned by the transport
// if the environment variable configuration is invalid. The Transport returned
// is always non-nil.
func InitDefault() (Transport, error) {
	t, err := getDefault()
	if apmdebug.TraceTransport {
		t = &debugTransport{transport: t}
	}
	Default = t
	return t, err
}

func getDefault() (Transport, error) {
	url := os.Getenv(envServerURL)
	if url == "" {
		return Discard, nil
	}
	t, err := NewHTTPTransport(url, "")
	if err != nil {
		return discardTransport{err}, err
	}
	return t, nil
}
