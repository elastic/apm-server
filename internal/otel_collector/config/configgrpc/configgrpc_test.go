// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configgrpc

import (
	"context"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	otelcol "go.opentelemetry.io/collector/internal/data/opentelemetry-proto-gen/collector/trace/v1"
	"go.opentelemetry.io/collector/testutil"
)

func TestDefaultGrpcClientSettings(t *testing.T) {
	gcs := &GRPCClientSettings{
		TLSSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
	}
	opts, err := gcs.ToDialOptions()
	assert.NoError(t, err)
	assert.Len(t, opts, 1)
}

func TestAllGrpcClientSettings(t *testing.T) {
	gcs := &GRPCClientSettings{
		Headers: map[string]string{
			"test": "test",
		},
		Endpoint:    "localhost:1234",
		Compression: "gzip",
		TLSSetting: configtls.TLSClientSetting{
			Insecure: false,
		},
		Keepalive: &KeepaliveClientConfig{
			Time:                time.Second,
			Timeout:             time.Second,
			PermitWithoutStream: true,
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		WaitForReady:    true,
		PerRPCAuth:      nil,
		BalancerName:    "round_robin",
	}
	opts, err := gcs.ToDialOptions()
	assert.NoError(t, err)
	assert.Len(t, opts, 6)
}

func TestDefaultGrpcServerSettings(t *testing.T) {
	gss := &GRPCServerSettings{}
	opts, err := gss.ToServerOption()
	assert.NoError(t, err)
	assert.Len(t, opts, 0)
}

func TestAllGrpcServerSettingsExceptAuth(t *testing.T) {
	gss := &GRPCServerSettings{
		NetAddr: confignet.NetAddr{
			Endpoint:  "localhost:1234",
			Transport: "tcp",
		},
		TLSSetting: &configtls.TLSServerSetting{
			TLSSetting:   configtls.TLSSetting{},
			ClientCAFile: "",
		},
		MaxRecvMsgSizeMiB:    1,
		MaxConcurrentStreams: 1024,
		ReadBufferSize:       1024,
		WriteBufferSize:      1024,
		Keepalive: &KeepaliveServerConfig{
			ServerParameters: &KeepaliveServerParameters{
				MaxConnectionIdle:     time.Second,
				MaxConnectionAge:      time.Second,
				MaxConnectionAgeGrace: time.Second,
				Time:                  time.Second,
				Timeout:               time.Second,
			},
			EnforcementPolicy: &KeepaliveEnforcementPolicy{
				MinTime:             time.Second,
				PermitWithoutStream: true,
			},
		},
	}
	opts, err := gss.ToServerOption()
	assert.NoError(t, err)
	assert.Len(t, opts, 7)
}

func TestGrpcServerAuthSettings(t *testing.T) {
	gss := &GRPCServerSettings{}

	// sanity check
	_, err := gss.ToServerOption()
	require.NoError(t, err)

	// test
	gss.Auth = &configauth.Authentication{
		OIDC: &configauth.OIDC{},
	}
	opts, err := gss.ToServerOption()

	// verify
	// an error here is a positive confirmation that Auth kicked in
	assert.Error(t, err)
	assert.Nil(t, opts)
}

func TestGRPCClientSettingsError(t *testing.T) {
	tests := []struct {
		settings GRPCClientSettings
		err      string
	}{
		{
			err: "^failed to load TLS config: failed to load CA CertPool: failed to load CA /doesnt/exist:",
			settings: GRPCClientSettings{
				Headers:     nil,
				Endpoint:    "",
				Compression: "",
				TLSSetting: configtls.TLSClientSetting{
					TLSSetting: configtls.TLSSetting{
						CAFile: "/doesnt/exist",
					},
					Insecure:   false,
					ServerName: "",
				},
				Keepalive: nil,
			},
		},
		{
			err: "^failed to load TLS config: for auth via TLS, either both certificate and key must be supplied, or neither",
			settings: GRPCClientSettings{
				Headers:     nil,
				Endpoint:    "",
				Compression: "",
				TLSSetting: configtls.TLSClientSetting{
					TLSSetting: configtls.TLSSetting{
						CertFile: "/doesnt/exist",
					},
					Insecure:   false,
					ServerName: "",
				},
				Keepalive: nil,
			},
		},
		{
			err: "invalid balancer_name: test",
			settings: GRPCClientSettings{
				Headers: map[string]string{
					"test": "test",
				},
				Endpoint:    "localhost:1234",
				Compression: "gzip",
				TLSSetting: configtls.TLSClientSetting{
					Insecure: false,
				},
				Keepalive: &KeepaliveClientConfig{
					Time:                time.Second,
					Timeout:             time.Second,
					PermitWithoutStream: true,
				},
				ReadBufferSize:  1024,
				WriteBufferSize: 1024,
				WaitForReady:    true,
				BalancerName:    "test",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.err, func(t *testing.T) {
			opts, err := test.settings.ToDialOptions()
			assert.Nil(t, opts)
			assert.Error(t, err)
			assert.Regexp(t, test.err, err)
		})
	}
}

func TestUseSecure(t *testing.T) {
	gcs := &GRPCClientSettings{
		Headers:     nil,
		Endpoint:    "",
		Compression: "",
		TLSSetting:  configtls.TLSClientSetting{},
		Keepalive:   nil,
		PerRPCAuth:  nil,
	}
	dialOpts, err := gcs.ToDialOptions()
	assert.NoError(t, err)
	assert.Equal(t, len(dialOpts), 1)
}

func TestGRPCServerSettingsError(t *testing.T) {
	tests := []struct {
		settings GRPCServerSettings
		err      string
	}{
		{
			err: "^failed to load TLS config: failed to load CA CertPool: failed to load CA /doesnt/exist:",
			settings: GRPCServerSettings{
				NetAddr: confignet.NetAddr{
					Endpoint:  "127.0.0.1:1234",
					Transport: "tcp",
				},
				TLSSetting: &configtls.TLSServerSetting{
					TLSSetting: configtls.TLSSetting{
						CAFile: "/doesnt/exist",
					},
				},
			},
		},
		{
			err: "^failed to load TLS config: for auth via TLS, either both certificate and key must be supplied, or neither",
			settings: GRPCServerSettings{
				NetAddr: confignet.NetAddr{
					Endpoint:  "127.0.0.1:1234",
					Transport: "tcp",
				},
				TLSSetting: &configtls.TLSServerSetting{
					TLSSetting: configtls.TLSSetting{
						CertFile: "/doesnt/exist",
					},
				},
			},
		},
		{
			err: "^failed to load TLS config: failed to load client CA CertPool: failed to load CA /doesnt/exist:",
			settings: GRPCServerSettings{
				NetAddr: confignet.NetAddr{
					Endpoint:  "127.0.0.1:1234",
					Transport: "tcp",
				},
				TLSSetting: &configtls.TLSServerSetting{
					ClientCAFile: "/doesnt/exist",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.err, func(t *testing.T) {
			_, err := test.settings.ToServerOption()
			assert.Regexp(t, test.err, err)
		})
	}
}

func TestGRPCServerSettings_ToListener_Error(t *testing.T) {
	settings := GRPCServerSettings{
		NetAddr: confignet.NetAddr{
			Endpoint:  "127.0.0.1:1234567",
			Transport: "tcp",
		},
		TLSSetting: &configtls.TLSServerSetting{
			TLSSetting: configtls.TLSSetting{
				CertFile: "/doesnt/exist",
			},
		},
		Keepalive: nil,
	}
	_, err := settings.ToListener()
	assert.Error(t, err)
}

func TestGetGRPCCompressionKey(t *testing.T) {
	if GetGRPCCompressionKey("gzip") != CompressionGzip {
		t.Error("gzip is marked as supported but returned unsupported")
	}

	if GetGRPCCompressionKey("Gzip") != CompressionGzip {
		t.Error("Capitalization of CompressionGzip should not matter")
	}

	if GetGRPCCompressionKey("badType") != CompressionUnsupported {
		t.Error("badType is not supported but was returned as supported")
	}
}

func TestHttpReception(t *testing.T) {
	tests := []struct {
		name           string
		tlsServerCreds *configtls.TLSServerSetting
		tlsClientCreds *configtls.TLSClientSetting
		hasError       bool
	}{
		{
			name:           "noTLS",
			tlsServerCreds: nil,
			tlsClientCreds: &configtls.TLSClientSetting{
				Insecure: true,
			},
		},
		{
			name: "TLS",
			tlsServerCreds: &configtls.TLSServerSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile:   path.Join(".", "testdata", "ca.crt"),
					CertFile: path.Join(".", "testdata", "server.crt"),
					KeyFile:  path.Join(".", "testdata", "server.key"),
				},
			},
			tlsClientCreds: &configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: path.Join(".", "testdata", "ca.crt"),
				},
				ServerName: "localhost",
			},
		},
		{
			name: "NoServerCertificates",
			tlsServerCreds: &configtls.TLSServerSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: path.Join(".", "testdata", "ca.crt"),
				},
			},
			tlsClientCreds: &configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: path.Join(".", "testdata", "ca.crt"),
				},
				ServerName: "localhost",
			},
			hasError: true,
		},
		{
			name: "mTLS",
			tlsServerCreds: &configtls.TLSServerSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile:   path.Join(".", "testdata", "ca.crt"),
					CertFile: path.Join(".", "testdata", "server.crt"),
					KeyFile:  path.Join(".", "testdata", "server.key"),
				},
				ClientCAFile: path.Join(".", "testdata", "ca.crt"),
			},
			tlsClientCreds: &configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile:   path.Join(".", "testdata", "ca.crt"),
					CertFile: path.Join(".", "testdata", "client.crt"),
					KeyFile:  path.Join(".", "testdata", "client.key"),
				},
				ServerName: "localhost",
			},
		},
		{
			name: "NoClientCertificate",
			tlsServerCreds: &configtls.TLSServerSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile:   path.Join(".", "testdata", "ca.crt"),
					CertFile: path.Join(".", "testdata", "server.crt"),
					KeyFile:  path.Join(".", "testdata", "server.key"),
				},
				ClientCAFile: path.Join(".", "testdata", "ca.crt"),
			},
			tlsClientCreds: &configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: path.Join(".", "testdata", "ca.crt"),
				},
				ServerName: "localhost",
			},
			hasError: true,
		},
		{
			name: "WrongClientCA",
			tlsServerCreds: &configtls.TLSServerSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile:   path.Join(".", "testdata", "ca.crt"),
					CertFile: path.Join(".", "testdata", "server.crt"),
					KeyFile:  path.Join(".", "testdata", "server.key"),
				},
				ClientCAFile: path.Join(".", "testdata", "server.crt"),
			},
			tlsClientCreds: &configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile:   path.Join(".", "testdata", "ca.crt"),
					CertFile: path.Join(".", "testdata", "client.crt"),
					KeyFile:  path.Join(".", "testdata", "client.key"),
				},
				ServerName: "localhost",
			},
			hasError: true,
		},
	}
	// prepare

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gss := &GRPCServerSettings{
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:0",
					Transport: "tcp",
				},
				TLSSetting: tt.tlsServerCreds,
			}
			ln, err := gss.ToListener()
			assert.NoError(t, err)
			opts, err := gss.ToServerOption()
			assert.NoError(t, err)
			s := grpc.NewServer(opts...)
			otelcol.RegisterTraceServiceServer(s, &grpcTraceServer{})

			go func() {
				_ = s.Serve(ln)
			}()

			gcs := &GRPCClientSettings{
				Endpoint:   ln.Addr().String(),
				TLSSetting: *tt.tlsClientCreds,
			}
			clientOpts, errClient := gcs.ToDialOptions()
			assert.NoError(t, errClient)
			grpcClientConn, errDial := grpc.Dial(gcs.Endpoint, clientOpts...)
			assert.NoError(t, errDial)
			client := otelcol.NewTraceServiceClient(grpcClientConn)
			ctx, cancelFunc := context.WithTimeout(context.Background(), 2*time.Second)
			resp, errResp := client.Export(ctx, &otelcol.ExportTraceServiceRequest{}, grpc.WaitForReady(true))
			if tt.hasError {
				assert.Error(t, errResp)
			} else {
				assert.NoError(t, errResp)
				assert.NotNil(t, resp)
			}
			cancelFunc()
			s.Stop()
		})
	}
}

func TestReceiveOnUnixDomainSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}
	socketName := testutil.TempSocketName(t)
	gss := &GRPCServerSettings{
		NetAddr: confignet.NetAddr{
			Endpoint:  socketName,
			Transport: "unix",
		},
	}
	ln, err := gss.ToListener()
	assert.NoError(t, err)
	opts, err := gss.ToServerOption()
	assert.NoError(t, err)
	s := grpc.NewServer(opts...)
	otelcol.RegisterTraceServiceServer(s, &grpcTraceServer{})

	go func() {
		_ = s.Serve(ln)
	}()

	gcs := &GRPCClientSettings{
		Endpoint: "unix://" + ln.Addr().String(),
		TLSSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
	}
	clientOpts, errClient := gcs.ToDialOptions()
	assert.NoError(t, errClient)
	grpcClientConn, errDial := grpc.Dial(gcs.Endpoint, clientOpts...)
	assert.NoError(t, errDial)
	client := otelcol.NewTraceServiceClient(grpcClientConn)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 2*time.Second)
	resp, errResp := client.Export(ctx, &otelcol.ExportTraceServiceRequest{}, grpc.WaitForReady(true))
	assert.NoError(t, errResp)
	assert.NotNil(t, resp)
	cancelFunc()
	s.Stop()
}

type grpcTraceServer struct{}

func (gts *grpcTraceServer) Export(context.Context, *otelcol.ExportTraceServiceRequest) (*otelcol.ExportTraceServiceResponse, error) {
	return &otelcol.ExportTraceServiceResponse{}, nil
}

func TestWithPerRPCAuthBearerToken(t *testing.T) {
	// prepare
	// test
	gcs := &GRPCClientSettings{
		PerRPCAuth: &PerRPCAuthConfig{
			AuthType:    "bearer",
			BearerToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
		},
	}
	dialOpts, err := gcs.ToDialOptions()

	// verify
	assert.NoError(t, err)
	assert.Len(t, dialOpts, 2) // WithInsecure and WithPerRPCCredentials
}

func TestWithPerRPCAuthInvalidAuthType(t *testing.T) {
	// test
	gcs := &GRPCClientSettings{
		PerRPCAuth: &PerRPCAuthConfig{
			AuthType: "non-existing",
		},
	}
	dialOpts, err := gcs.ToDialOptions()

	// verify
	assert.Error(t, err)
	assert.Nil(t, dialOpts)
}
