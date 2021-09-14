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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/logp/configure"
)

func TestLogging(t *testing.T) {
	methodName := "test_method_name"
	info := &grpc.UnaryServerInfo{
		FullMethod: methodName,
	}
	ctx := ContextWithClientMetadata(context.Background(), ClientMetadataValues{
		SourceAddr: &net.TCPAddr{
			IP:   net.ParseIP("1.2.3.4"),
			Port: 4321,
		},
	})

	requiredKeys := []string{
		"source.address",
		"grpc.request.method",
		"event.duration",
		"grpc.response.status_code",
	}

	for _, tc := range []struct {
		statusCode codes.Code
		f          func(ctx context.Context, req interface{}) (interface{}, error)
	}{
		{
			statusCode: codes.Internal,
			f: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, status.New(codes.Internal, "internal server error").Err()
			},
		},
		{
			statusCode: codes.OK,
			f: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, nil
			},
		},
	} {
		configure.Logging(
			"APM Server test",
			common.MustNewConfigFrom(`{"ecs":true}`),
		)
		require.NoError(t, logp.DevelopmentSetup(logp.ToObserverOutput()))
		logger := logp.NewLogger("interceptor.logging.test")

		i := Logging(logger)
		_, err := i(ctx, nil, info, tc.f)
		entries := logp.ObserverLogs().TakeAll()
		assert.Len(t, entries, 1)
		entry := entries[0]

		fields := entry.ContextMap()
		if tc.statusCode != codes.OK {
			assert.Error(t, err)
			assert.Equal(t, zapcore.ErrorLevel, entry.Entry.Level)
			assert.Equal(t, "internal server error", fields["error.message"])
		} else {
			assert.NoError(t, err)
			assert.Equal(t, zapcore.InfoLevel, entry.Entry.Level)
			assert.NotContains(t, fields, "error.message")
		}
		for _, k := range requiredKeys {
			assert.Contains(t, fields, k)
		}
		assert.Equal(t, methodName, fields["grpc.request.method"])
		assert.Equal(t, "1.2.3.4:4321", fields["source.address"])
		assert.Equal(t, tc.statusCode.String(), fields["grpc.response.status_code"])
	}
}
