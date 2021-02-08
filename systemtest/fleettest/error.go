package fleettest

// Error is an error type returned by Client methods on failed
// requests to the Kibana Fleet API.
type Error struct {
	StatusCode int    `json:"statusCode"`
	ErrorCode  string `json:"error"`
	Message    string `json:"message"`
}

// Error returns the error message.
func (e *Error) Error() string {
	return e.Message
}
