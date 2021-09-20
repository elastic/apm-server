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
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/elastic/beats/v7/libbeat/logp"
)

// Logging intercepts a gRPC request and provides logging processing. The
// returned function implements grpc.UnaryServerInterceptor.
//
// Logging should be added after ClientMetadata to include `source.address`
// in log records.
func Logging(logger *logp.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Shadow the logger param to ensure we don't update the
		// closure variable, and interfere with logging of other
		// requests.
		logger := logger

		start := time.Now()
		if metadata, ok := ClientMetadataFromContext(ctx); ok {
			if metadata.SourceAddr != nil {
				logger = logger.With("source.address", metadata.SourceAddr.String())
			}
		}

		resp, err := handler(ctx, req)
		res, _ := status.FromError(err)
		logger = logger.With(
			"grpc.request.method", info.FullMethod,
			"event.duration", time.Since(start),
			"grpc.response.status_code", res.Code(),
		)

		if err != nil {
			logger.With("error.message", res.Message()).Error(logp.Error(err))
		} else {
			logger.Info(res.Message())
		}
		return resp, err
	}
}
