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

package interceptors_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/beater/interceptors"
)

func TestTimeout(t *testing.T) {
	for _, tc := range []struct {
		err     error
		timeout bool
	}{{
		err: nil,
	}, {
		err: errors.New("not timeout"),
	}, {
		err:     context.Canceled,
		timeout: true,
	}, {
		err:     context.DeadlineExceeded,
		timeout: true,
	}, {
		err:     fmt.Errorf("wrapped: %w", context.Canceled),
		timeout: true,
	}, {
		err:     errors.Wrap(context.DeadlineExceeded, "also wrapped"),
		timeout: true,
	}} {
		interceptor := interceptors.Timeout()
		resp, err := interceptor(context.Background(), "request_arg", &grpc.UnaryServerInfo{},
			func(context.Context, interface{}) (interface{}, error) { return 123, tc.err },
		)
		if tc.err == nil {
			assert.NoError(t, err)
		} else if !tc.timeout {
			assert.Equal(t, tc.err, err)
		} else {
			s, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, codes.DeadlineExceeded, s.Code())
			assert.Equal(t, "request timed out", s.Message())
		}
		assert.Equal(t, 123, resp) // always returned unchanged by interceptor
	}
}
