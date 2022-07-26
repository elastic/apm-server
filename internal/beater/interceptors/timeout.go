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

package interceptors

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Timeout returns a grpc.UnaryServerInterceptor that intercepts
// context.Canceled and context.DeadlineExceeded errors, and
// updates the response to indicate that the request timed out.
// This could be caused by either a client timeout or server timeout.
func Timeout() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		resp, err := handler(ctx, req)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if s, ok := status.FromError(err); !ok || s.Code() == codes.OK {
				err = status.Error(codes.DeadlineExceeded, "request timed out")
			}
		}
		return resp, err
	}
}
