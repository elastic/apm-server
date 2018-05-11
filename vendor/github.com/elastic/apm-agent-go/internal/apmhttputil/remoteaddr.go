package apmhttputil

import (
	"net"
	"net/http"
	"strings"
)

// RemoteAddr returns the remote address for the HTTP request.
//
// In order:
//  - if the Forwarded header is set, then the first item in the
//    list's "for" field is used, if it exists. The "for" value
//    is returned even if it is an obfuscated identifier.
//  - if the X-Real-Ip header is set, then its value is returned.
//  - if the X-Forwarded-For header is set, then the first value
//    in the comma-separated list is returned.
//  - otherwise, the host portion of req.RemoteAddr is returned.
func RemoteAddr(req *http.Request, forwarded *ForwardedHeader) string {
	if forwarded != nil {
		if forwarded.For != "" {
			remoteAddr, _, err := net.SplitHostPort(forwarded.For)
			if err != nil {
				remoteAddr = forwarded.For
			}
			return remoteAddr
		}
	}
	if realIP := req.Header.Get("X-Real-Ip"); realIP != "" {
		return realIP
	}
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		if sep := strings.IndexRune(xff, ','); sep > 0 {
			xff = xff[:sep]
		}
		return strings.TrimSpace(xff)
	}
	remoteAddr, _ := splitHost(req.RemoteAddr)
	return remoteAddr
}
