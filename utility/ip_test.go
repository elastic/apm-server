package utility

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractIP(t *testing.T) {
	var req = func(real *string, forward *string) *http.Request {
		req, _ := http.NewRequest("POST", "_", nil)
		req.RemoteAddr = "10.11.12.13:8080"
		if real != nil {
			req.Header.Add("X-Real-IP", *real)
		}
		if forward != nil {
			req.Header.Add("X-Forwarded-For", *forward)
		}
		return req
	}

	real := "54.55.101.102"
	assert.Equal(t, real, ExtractIP(req(&real, nil)))

	forwardedFor := "54.56.103.104"
	assert.Equal(t, real, ExtractIP(req(&real, &forwardedFor)))
	assert.Equal(t, forwardedFor, ExtractIP(req(nil, &forwardedFor)))

	forwardedForMultiple := "54.56.103.104 , 54.57.105.106 , 54.58.107.108"
	assert.Equal(t, forwardedFor, ExtractIP(req(nil, &forwardedForMultiple)))

	assert.Equal(t, "10.11.12.13", ExtractIP(req(nil, nil)))
	assert.Equal(t, "10.11.12.13", ExtractIP(req(new(string), new(string))))
}
