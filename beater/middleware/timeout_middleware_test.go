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
	"net/http"
	"net/http/httptest"
	"testing"

<<<<<<< HEAD
<<<<<<< HEAD
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/request"
=======
	"github.com/elastic/apm-server/beater/request"
	"github.com/stretchr/testify/assert"
>>>>>>> 6c695c6d7... modify result via middleware
=======
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/request"
>>>>>>> 24554c77e... gofmt
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
