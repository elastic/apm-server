package apmhttputil

import (
	"net"
	"net/http"
	"strings"

	"github.com/elastic/apm-agent-go/internal/apmstrings"
	"github.com/elastic/apm-agent-go/model"
)

// RequestURL returns a model.URL for req.
//
// If req contains an absolute URI, the values will be split and
// sanitized, but no further processing performed. For all other
// requests (i.e. most server-side requests), we reconstruct the
// URL based on various proxy forwarding headers and other request
// attributes.
func RequestURL(req *http.Request, forwarded *ForwardedHeader) model.URL {
	out := model.URL{
		Path:   truncateString(req.URL.Path),
		Search: truncateString(req.URL.RawQuery),
		Hash:   truncateString(req.URL.Fragment),
	}
	if req.URL.Host != "" {
		// Absolute URI: client-side or proxy request, so ignore the
		// headers.
		hostname, port := splitHost(req.URL.Host)
		out.Hostname = truncateString(hostname)
		out.Port = truncateString(port)
		out.Protocol = truncateString(req.URL.Scheme)
		return out
	}

	// This is a server-side request URI, which contains only the path.
	// We synthesize the full URL by extracting the host and protocol
	// from headers, or inferring from other properties.
	var fullHost string
	if forwarded != nil && forwarded.Host != "" {
		fullHost = forwarded.Host
		out.Protocol = truncateString(forwarded.Proto)
	} else if xfh := req.Header.Get("X-Forwarded-Host"); xfh != "" {
		fullHost = xfh
	} else {
		fullHost = req.Host
	}
	hostname, port := splitHost(fullHost)
	out.Hostname = truncateString(hostname)
	out.Port = truncateString(port)

	// Protocol might be extracted from the Forwarded header. If it's not,
	// look for various other headers.
	if out.Protocol == "" {
		if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
			out.Protocol = truncateString(proto)
		} else if proto := req.Header.Get("X-Forwarded-Protocol"); proto != "" {
			out.Protocol = truncateString(proto)
		} else if proto := req.Header.Get("X-Url-Scheme"); proto != "" {
			out.Protocol = truncateString(proto)
		} else if req.Header.Get("Front-End-Https") == "on" {
			out.Protocol = "https"
		} else if req.Header.Get("X-Forwarded-Ssl") == "on" {
			out.Protocol = "https"
		} else if req.TLS != nil {
			out.Protocol = "https"
		} else {
			// Assume http otherwise.
			out.Protocol = "http"
		}
	}
	return out
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

func truncateString(s string) string {
	// At the time of writing, all length limits are 1024.
	return apmstrings.Truncate(s, 1024)
}
