package apmhttputil_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/internal/apmhttputil"
)

func TestRemoteAddr(t *testing.T) {
	req := &http.Request{
		RemoteAddr: "[::1]:1234",
		Header:     make(http.Header),
	}
	assert.Equal(t, "::1", apmhttputil.RemoteAddr(req, nil))

	req.Header.Set("X-Forwarded-For", "client.invalid")
	assert.Equal(t, "client.invalid", apmhttputil.RemoteAddr(req, nil))

	req.Header.Set("X-Forwarded-For", "client.invalid, proxy.invalid")
	assert.Equal(t, "client.invalid", apmhttputil.RemoteAddr(req, nil))

	req.Header.Set("X-Real-IP", "127.1.2.3")
	assert.Equal(t, "127.1.2.3", apmhttputil.RemoteAddr(req, nil))

	// "for" is missing from Forwarded, so fall back to the next thing
	assert.Equal(t, "127.1.2.3", apmhttputil.RemoteAddr(req, &apmhttputil.ForwardedHeader{}))

	assert.Equal(t, "_secret", apmhttputil.RemoteAddr(req, &apmhttputil.ForwardedHeader{For: "_secret"}))

	assert.Equal(t, "2001:db8:cafe::17", apmhttputil.RemoteAddr(req, &apmhttputil.ForwardedHeader{
		For: "[2001:db8:cafe::17]:4711",
	}))
}
