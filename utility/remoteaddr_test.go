package utility_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/utility"
)

func TestRemoteAddr(t *testing.T) {
	req := &http.Request{
		RemoteAddr: "[::1]:1234",
		Header:     make(http.Header),
	}
	assert.Equal(t, "::1", utility.RemoteAddr(req))

	req.Header.Set("X-Forwarded-For", "client.invalid")
	assert.Equal(t, "client.invalid", utility.RemoteAddr(req))

	req.Header.Set("X-Forwarded-For", "client.invalid, proxy.invalid")
	assert.Equal(t, "client.invalid", utility.RemoteAddr(req))

	req.Header.Set("X-Real-IP", "127.1.2.3")
	assert.Equal(t, "127.1.2.3", utility.RemoteAddr(req))

	// "for" is missing from Forwarded, so fall back to the next thing
	req.Header.Set("Forwarded", "")
	assert.Equal(t, "127.1.2.3", utility.RemoteAddr(req))

	req.Header.Set("Forwarded", "for=_secret")
	assert.Equal(t, "_secret", utility.RemoteAddr(req))

	req.Header.Set("Forwarded", "for=[2001:db8:cafe::17]:4711")
	assert.Equal(t, "2001:db8:cafe::17", utility.RemoteAddr(req))
}
