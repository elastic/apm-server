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
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/v7/libbeat/logp"
)

// Logging intercepts a gRPC request and provides logging processing. The
// returned function implements grpc.UnaryServerInterceptor.
func Logging(logger *logp.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		start := time.Now()
		if p, ok := peer.FromContext(ctx); ok {
			logger = logger.With("source.address", p.Addr)
		}
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			headers := http.Header(md)
			// Account for `Forwarded`, `X-Real-IP`, `X-Forwarded-For` headers
			if ip := utility.ExtractIPFromHeader(headers); ip != nil {
				logger = logger.With("source.address", ip)
			}
		}

		resp, err = handler(ctx, req)
		res, _ := status.FromError(err)
		logger = logger.With(
			"grpc.request.method", info.FullMethod,
			"event.duration", time.Since(start),
			"grpc.response.status_code", res.Code(),
		)

		if err != nil {
			logger.With("error.message", res.Message()).Error(logp.Error(err))
		}
		return
	}
}
