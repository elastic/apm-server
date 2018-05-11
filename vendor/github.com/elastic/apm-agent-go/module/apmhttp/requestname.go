// +build go1.10

package apmhttp

import (
	"net/http"
	"strings"
)

// ServerRequestName returns the transaction name for the server request, req.
func ServerRequestName(req *http.Request) string {
	var b strings.Builder
	b.Grow(len(req.Method) + len(req.URL.Path) + 1)
	b.WriteString(req.Method)
	b.WriteByte(' ')
	b.WriteString(req.URL.Path)
	return b.String()
}

// ClientRequestName returns the span name for the client request, req.
func ClientRequestName(req *http.Request) string {
	var b strings.Builder
	b.Grow(len(req.Method) + len(req.URL.Host) + 1)
	b.WriteString(req.Method)
	b.WriteByte(' ')
	b.WriteString(req.URL.Host)
	return b.String()
}
