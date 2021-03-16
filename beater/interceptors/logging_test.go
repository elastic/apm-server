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
<<<<<<< HEAD
=======
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
>>>>>>> 17433dac9... add logging to jaeger and otlp gRPC calls (#4934)
	"google.golang.org/grpc/status"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/logp/configure"
)

func TestLogging(t *testing.T) {
<<<<<<< HEAD
	ip1234 := net.ParseIP("1.2.3.4")
=======
	addr := &net.TCPAddr{
		IP:   net.ParseIP("1.2.3.4"),
		Port: 56837,
	}
	p := &peer.Peer{
		Addr: addr,
	}

	ctx := peer.NewContext(context.Background(), p)
>>>>>>> 17433dac9... add logging to jaeger and otlp gRPC calls (#4934)
	methodName := "test_method_name"
	info := &grpc.UnaryServerInfo{
		FullMethod: methodName,
	}
<<<<<<< HEAD
	ctx := ContextWithClientMetadata(context.Background(), ClientMetadataValues{
		SourceIP: ip1234,
	})
=======
>>>>>>> 17433dac9... add logging to jaeger and otlp gRPC calls (#4934)

	requiredKeys := []string{
		"source.address",
		"grpc.request.method",
		"event.duration",
		"grpc.response.status_code",
	}

	for _, tc := range []struct {
<<<<<<< HEAD
		statusCode codes.Code
		f          func(ctx context.Context, req interface{}) (interface{}, error)
=======
		alternateIP string
		statusCode  codes.Code
		f           func(ctx context.Context, req interface{}) (interface{}, error)
>>>>>>> 17433dac9... add logging to jaeger and otlp gRPC calls (#4934)
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
<<<<<<< HEAD
=======
		{
			alternateIP: "6.7.8.9",
			statusCode:  codes.OK,
			f: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, nil
			},
		},
>>>>>>> 17433dac9... add logging to jaeger and otlp gRPC calls (#4934)
	} {
		configure.Logging(
			"APM Server test",
			common.MustNewConfigFrom(`{"ecs":true}`),
		)
		require.NoError(t, logp.DevelopmentSetup(logp.ToObserverOutput()))
		logger := logp.NewLogger("interceptor.logging.test")

<<<<<<< HEAD
=======
		wantAddr := addr.String()
		if tc.alternateIP != "" {
			wantAddr = tc.alternateIP
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("X-Real-IP", tc.alternateIP))
		}

>>>>>>> 17433dac9... add logging to jaeger and otlp gRPC calls (#4934)
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
<<<<<<< HEAD
		assert.Equal(t, ip1234.String(), fields["source.address"])
=======
		assert.Equal(t, wantAddr, fields["source.address"])
>>>>>>> 17433dac9... add logging to jaeger and otlp gRPC calls (#4934)
		assert.Equal(t, tc.statusCode.String(), fields["grpc.response.status_code"])
	}
}
