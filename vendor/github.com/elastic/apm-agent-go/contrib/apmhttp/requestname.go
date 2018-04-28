// +build go1.10

package apmhttp

import (
	"net/http"
	"strings"
)

// RequestName returns the transaction name for req.
func RequestName(req *http.Request) string {
	var b strings.Builder
	b.Grow(len(req.Method) + len(req.URL.Path) + 1)
	b.WriteString(req.Method)
	b.WriteByte(' ')
	b.WriteString(req.URL.Path)
	return b.String()
}
