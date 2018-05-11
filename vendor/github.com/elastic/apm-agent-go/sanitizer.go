package elasticapm

import (
	"bytes"
	"regexp"

	"github.com/elastic/apm-agent-go/model"
)

const redacted = "[REDACTED]"

// sanitizeRequest sanitizes HTTP request data, redacting
// the values of cookies and forms whose corresponding keys
// match the given regular expression.
func sanitizeRequest(r *model.Request, re *regexp.Regexp) {
	var anyCookiesRedacted bool
	for _, c := range r.Cookies {
		if !re.MatchString(c.Name) {
			continue
		}
		c.Value = redacted
		anyCookiesRedacted = true
	}
	if anyCookiesRedacted && r.Headers != nil {
		var b bytes.Buffer
		for i, c := range r.Cookies {
			if i != 0 {
				b.WriteRune(';')
			}
			b.WriteString(c.String())
		}
		r.Headers.Cookie = b.String()
	}
	if r.Body != nil && r.Body.Form != nil {
		for key, values := range r.Body.Form {
			if !re.MatchString(key) {
				continue
			}
			for i := range values {
				values[i] = redacted
			}
		}
	}
}
