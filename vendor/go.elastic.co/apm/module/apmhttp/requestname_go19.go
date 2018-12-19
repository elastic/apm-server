// +build !go1.10

package apmhttp

import "net/http"

// ServerRequestName returns the transaction name for the server request, req.
func ServerRequestName(req *http.Request) string {
	buf := make([]byte, len(req.Method)+len(req.URL.Path)+1)
	n := copy(buf, req.Method)
	buf[n] = ' '
	copy(buf[n+1:], req.URL.Path)
	return string(buf)
}

// ClientRequestName returns the span name for the client request, req.
func ClientRequestName(req *http.Request) string {
	buf := make([]byte, len(req.Method)+len(req.URL.Host)+1)
	n := copy(buf, req.Method)
	buf[n] = ' '
	copy(buf[n+1:], req.URL.Host)
	return string(buf)
}
