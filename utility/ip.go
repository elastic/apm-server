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
	var remoteAddr = func() string {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return ip
	}

	var forwarded = func() string {
		forwardedFor := r.Header.Get("X-Forwarded-For")
		client := strings.Split(forwardedFor, ",")[0]
		return strings.TrimSpace(client)
	}

	var real = func() string {
		return r.Header.Get("X-Real-IP")
	}

	if ip := real(); ip != "" {
		return ip
	}
	if ip := forwarded(); ip != "" {
		return ip
	}
	return remoteAddr()
}
