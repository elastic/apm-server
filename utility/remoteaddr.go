package utility

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
//
// Because the client can control the headers, they can control
// the result of this function. The result should therefore not
// necessarily be trusted to be correct.
func RemoteAddr(req *http.Request) string {
	if fwd := req.Header.Get("Forwarded"); fwd != "" {
		forwarded := ParseForwarded(fwd)
		if forwarded.For != "" {
			host, _ := splitHost(forwarded.For)
			return host
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
	host, _ := splitHost(req.RemoteAddr)
	return host
}

func splitHost(in string) (host, port string) {
	if strings.LastIndexByte(in, ':') == -1 {
		// In the common (relative to other "errors") case that
		// there is no colon, we can avoid allocations by not
		// calling SplitHostPort.
		return in, ""
	}
	host, port, err := net.SplitHostPort(in)
	if err != nil {
		return in, ""
	}
	return host, port
}
