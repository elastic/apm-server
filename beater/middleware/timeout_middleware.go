package middleware

import (
	"context"

	"github.com/elastic/apm-server/beater/request"
	"github.com/pkg/errors"
)

// TimeoutMiddleware assumes that a context.Canceled error indicates a timed out
// request. This could be caused by a either a client timeout or server timeout.
// The middleware sets the Context.Result.
func TimeoutMiddleware() Middleware {
	tErr := errors.New("request timed out")
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			h(c)

			err := c.Request.Context().Err()
			if errors.Is(err, context.Canceled) {
				id := request.IDResponseErrorsTimeout
				code := request.MapResultIDToStatus[id].Code

				c.Result.Set(id, code, request.MapResultIDToStatus[id].Keyword, tErr.Error(), tErr)
			}
		}, nil
	}
}
