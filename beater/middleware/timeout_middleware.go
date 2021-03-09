<<<<<<< HEAD
<<<<<<< HEAD
=======
>>>>>>> f918dd0aa... add license to new files
// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

<<<<<<< HEAD
=======
>>>>>>> 6c695c6d7... modify result via middleware
=======
>>>>>>> f918dd0aa... add license to new files
package middleware

import (
	"context"

<<<<<<< HEAD
<<<<<<< HEAD
	"github.com/pkg/errors"

	"github.com/elastic/apm-server/beater/request"
=======
	"github.com/elastic/apm-server/beater/request"
	"github.com/pkg/errors"
>>>>>>> 6c695c6d7... modify result via middleware
=======
	"github.com/pkg/errors"

	"github.com/elastic/apm-server/beater/request"
>>>>>>> 24554c77e... gofmt
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
<<<<<<< HEAD
<<<<<<< HEAD
				c.Result.SetDefault(request.IDResponseErrorsTimeout)
				c.Result.Err = tErr
				c.Result.Body = tErr.Error()
=======
				id := request.IDResponseErrorsTimeout
				code := request.MapResultIDToStatus[id].Code

				c.Result.Set(id, code, request.MapResultIDToStatus[id].Keyword, tErr.Error(), tErr)
>>>>>>> 6c695c6d7... modify result via middleware
=======
				c.Result.SetDefault(request.IDResponseErrorsTimeout)
>>>>>>> 7cf450d1a... Update beater/middleware/timeout_middleware.go
			}
		}, nil
	}
}
