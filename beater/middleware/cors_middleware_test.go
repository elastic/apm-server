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

package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

func TestCORSMiddleware(t *testing.T) {

	cors := func(origin string, allowedOrigins, allowedHeaders []string, m string) (*request.Context, *httptest.ResponseRecorder) {
		c, rec := beatertest.ContextWithResponseRecorder(m, "/")
		c.Request.Header.Set(headers.Origin, origin)
		Apply(CORSMiddleware(allowedOrigins, allowedHeaders), beatertest.Handler202)(c)
		return c, rec
	}

	checkPreflightHeaders := func(t *testing.T, rec *httptest.ResponseRecorder) {
		assert.Equal(t, "3600", rec.Header().Get(headers.AccessControlMaxAge))
		assert.Equal(t, "Origin", rec.Header().Get(headers.Vary))
		assert.Equal(t, "POST, OPTIONS", rec.Header().Get(headers.AccessControlAllowMethods))
		assert.Equal(t, "Content-Type, Content-Encoding, Accept", rec.Header().Get(headers.AccessControlAllowHeaders))
		assert.Equal(t, "0", rec.Header().Get(headers.ContentLength))
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Body.String())
	}

	t.Run("OPTIONSValidOrigin", func(t *testing.T) {
		_, rec := cors("wxyz", []string{"w*yz"}, nil, http.MethodOptions)
		checkPreflightHeaders(t, rec)
		assert.Equal(t, "wxyz", rec.Header().Get(headers.AccessControlAllowOrigin))
	})
	t.Run("OPTIONSInvalidOrigin", func(t *testing.T) {
		_, rec := cors("xyz", []string{"xy.*i"}, nil, http.MethodOptions)
		checkPreflightHeaders(t, rec)
		assert.Empty(t, rec.Header().Get(headers.AccessControlAllowOrigin))
	})

	t.Run("GETAllowedOrigins", func(t *testing.T) {
		for _, origin := range []string{"", "wxyz", "testingx"} {
			_, rec := cors(origin, []string{"*", "testing.*"}, nil, http.MethodPost)

			assert.Equal(t, http.StatusAccepted, rec.Code)
			assert.Equal(t, origin, rec.Header().Get(headers.AccessControlAllowOrigin))
		}
	})

	t.Run("GETForbidden", func(t *testing.T) {
		for _, origin := range []string{"", "wxyz", "test"} {
			_, rec := cors(origin, []string{"xyz", "testing.*"}, nil, http.MethodPost)

			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Equal(t,
				beatertest.ResultErrWrap(fmt.Sprintf("%s: origin: '%s' is not allowed",
					request.MapResultIDToStatus[request.IDResponseErrorsForbidden].Keyword,
					origin)),
				rec.Body.String())
		}
	})

	t.Run("AllowedHeaders", func(t *testing.T) {
		_, rec := cors("xyz", []string{"xyz"}, []string{"Authorization"}, http.MethodOptions)
		assert.Contains(t, rec.Header().Get(headers.AccessControlAllowHeaders), "Authorization")
	})

}
