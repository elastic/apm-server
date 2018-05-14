package utility

import (
	"net"
	"net/http"
	"strings"
)

// Obtains the IP of a request, looking up X-Forwarded-For and X-Real-IP headers.
// X-Forwarded-For has a list of IPs, of which the first is the one of the original client.
// This value however might not be necessarily trusted, as it can be forged by a malicious user.
func ExtractIP(r *http.Request) string {
	if realIP := r.Header.Get("X-Real-Ip"); realIP != "" {
		return realIP
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if sep := strings.IndexRune(xff, ','); sep > 0 {
			xff = xff[:sep]
		}
		return strings.TrimSpace(xff)
	}
	remoteAddr, _ := splitHost(r.RemoteAddr)
	return remoteAddr
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
