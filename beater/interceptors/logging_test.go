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
	"fmt"
	"net"
	"testing"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/logp/configure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func TestLogging(t *testing.T) {
	addr := &net.TCPAddr{
		IP:   net.ParseIP("1.2.3.4"),
		Port: 56837,
	}
	p := &peer.Peer{
		Addr: addr,
	}

	ctx := peer.NewContext(context.Background(), p)
	methodName := "test_method_name"
	info := &grpc.UnaryServerInfo{
		FullMethod: methodName,
	}

	requiredKeys := []string{
		"source.address",
		"grpc.request.method",
		"event.duration",
		"grpc.response.status_code",
	}

	for _, tc := range []struct {
		alternateIP string
		statusCode  codes.Code
		f           func(ctx context.Context, req interface{}) (interface{}, error)
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
		{
			alternateIP: "6.7.8.9",
			statusCode:  codes.OK,
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

		wantAddr := addr.String()
		if tc.alternateIP != "" {
			wantAddr = tc.alternateIP
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("X-Real-IP", tc.alternateIP))
		}

		i := Logging(logger)
		_, err := i(ctx, nil, info, tc.f)
		entries := logp.ObserverLogs().TakeAll()
		assert.Len(t, entries, 1)
		entry := entries[0]
		logMap := newLogMap(entry)
		hasErr := tc.statusCode != codes.OK

		for _, k := range requiredKeys {
			ok, err := logMap.HasKey(k)
			assert.NoError(t, err)
			assert.True(t, ok)
		}

		gotMethod, _ := logMap.GetValue("grpc.request.method")
		assert.Equal(t, methodName, gotMethod)
		gotAddr, _ := logMap.GetValue("source.address")
		assert.Equal(t, wantAddr, gotAddr)
		gotStatusCode, _ := logMap.GetValue("grpc.response.status_code")
		assert.Equal(t, fmt.Sprintf("%v", tc.statusCode), gotStatusCode)

		assert.Equal(t, hasErr, err != nil)
		ok, err := logMap.HasKey("error.message")
		assert.Equal(t, ok, hasErr)

		if hasErr {
			assert.Equal(t, "error", entry.Entry.Level.String())
			errMsg, _ := logMap.GetValue("error.message")
			assert.Equal(t, "internal server error", errMsg)
		} else {
			assert.Equal(t, "info", entry.Entry.Level.String())
		}
	}
}

func newLogMap(entry observer.LoggedEntry) common.MapStr {
	encoder := zapcore.NewMapObjectEncoder()
	ec := common.MapStr{}
	for _, f := range entry.Context {
		f.AddTo(encoder)
		ec.DeepUpdate(encoder.Fields)
	}
	return ec
}
