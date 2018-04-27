// +build !go1.10

package apmhttp

import "net/http"

// RequestName returns the transaction name for req.
func RequestName(req *http.Request) string {
	buf := make([]byte, len(req.Method)+len(req.URL.Path)+1)
	n := copy(buf, req.Method)
	buf[n] = ' '
	copy(buf[n+1:], req.URL.Path)
	return string(buf)
}
