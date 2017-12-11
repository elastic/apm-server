package utility

import (
	"net"
	"net/http"
	"strings"
)

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
