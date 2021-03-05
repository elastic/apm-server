package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/apm-server/beater/request"
	"github.com/stretchr/testify/assert"
)

func TestTimeoutMiddleware(t *testing.T) {
	var err error
	m := TimeoutMiddleware()
	h := request.Handler(func(c *request.Context) {
		ctx := c.Request.Context()
		ctx, cancel := context.WithCancel(ctx)
		r := c.Request.WithContext(ctx)
		c.Request = r
		cancel()
	})

	h, err = m(h)
	assert.NoError(t, err)

	c := request.NewContext()
	r, err := http.NewRequest("GET", "/", nil)
	assert.NoError(t, err)
	c.Reset(httptest.NewRecorder(), r)
	h(c)

	assert.Equal(t, http.StatusServiceUnavailable, c.Result.StatusCode)
}
