package apmhttputil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/internal/apmhttputil"
)

func TestParseForwarded(t *testing.T) {
	type test struct {
		name   string
		header string
		expect apmhttputil.ForwardedHeader
	}

	tests := []test{{
		name:   "Forwarded",
		header: "by=127.0.0.1; for=127.1.1.1; Host=\"forwarded.invalid:443\"; proto=HTTPS",
		expect: apmhttputil.ForwardedHeader{
			For:   "127.1.1.1",
			Host:  "forwarded.invalid:443",
			Proto: "HTTPS",
		},
	}, {
		name:   "Forwarded-Multi",
		header: "host=first.invalid, host=second.invalid",
		expect: apmhttputil.ForwardedHeader{
			Host: "first.invalid",
		},
	}, {
		name:   "Forwarded-Malformed-Fields-Ignored",
		header: "what; nonsense=\"; host=first.invalid",
		expect: apmhttputil.ForwardedHeader{
			Host: "first.invalid",
		},
	}, {
		name:   "Forwarded-Trailing-Separators",
		header: "host=first.invalid;,",
		expect: apmhttputil.ForwardedHeader{
			Host: "first.invalid",
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := apmhttputil.ParseForwarded(test.header)
			assert.Equal(t, test.expect, parsed)
		})
	}
}
