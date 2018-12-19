package apm

import (
	"go.elastic.co/apm/internal/wildcard"
	"go.elastic.co/apm/model"
)

const redacted = "[REDACTED]"

// sanitizeRequest sanitizes HTTP request data, redacting the
// values of cookies, headers and forms whose corresponding keys
// match any of the given wildcard patterns.
func sanitizeRequest(r *model.Request, matchers wildcard.Matchers) {
	for _, c := range r.Cookies {
		if !matchers.MatchAny(c.Name) {
			continue
		}
		c.Value = redacted
	}
	sanitizeHeaders(r.Headers, matchers)
	if r.Body != nil && r.Body.Form != nil {
		for key, values := range r.Body.Form {
			if !matchers.MatchAny(key) {
				continue
			}
			for i := range values {
				values[i] = redacted
			}
		}
	}
}

// sanitizeResponse sanitizes HTTP response data, redacting
// the values of response headers whose corresponding keys
// match any of the given wildcard patterns.
func sanitizeResponse(r *model.Response, matchers wildcard.Matchers) {
	sanitizeHeaders(r.Headers, matchers)
}

func sanitizeHeaders(headers model.Headers, matchers wildcard.Matchers) {
	for i := range headers {
		h := &headers[i]
		if !matchers.MatchAny(h.Key) || len(h.Values) == 0 {
			continue
		}
		h.Values = h.Values[:1]
		h.Values[0] = redacted
	}
}
