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

package elasticsearch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasPrivilegesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, "oh noes")
	}))
	defer server.Close()

	client, err := NewClient(&Config{Hosts: Hosts{server.Listener.Addr().String()}})
	require.NoError(t, err)

	resp, err := HasPrivileges(context.Background(), client, HasPrivilegesRequest{}, "foo")
	require.Error(t, err)
	assert.Zero(t, resp)

	var eserr *Error
	require.True(t, errors.As(err, &eserr))
	assert.Equal(t, 401, eserr.StatusCode)
	assert.Equal(t, "oh noes", eserr.Error())
}
