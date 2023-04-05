// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package profiling

import (
	"context"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/headers"
)

var (
	// This is the version of the gRPC protocol specified in collection_agent.proto.
	// It's retrieved and cached in NewCollector and used to check for protocol mismatches
	// against the protocol version that the client sends.
	rpcProtocolVersion uint32
)

// GetRPCVersion returns the version of the RPC protocol
func GetRPCVersion() uint32 {
	// Retrieve protocol version defined in the .proto file
	options := File_collection_agent_proto.Options()
	return proto.GetExtension(options, E_Version).(uint32)
}

// AuthenticateUnaryCall implements the interceptors.UnaryAuthenticator
// interface, extracting the secret token supplied by the Host Agent,
// which we will treat the same as an APM secret token.
func (e *ElasticCollector) AuthenticateUnaryCall(
	ctx context.Context,
	req interface{},
	fullMethod string,
	authenticator *auth.Authenticator,
) (auth.AuthenticationDetails, auth.Authorizer, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	secretToken := GetFirstOrEmpty(md, MetadataKeySecretToken)
	projectIDStr := GetFirstOrEmpty(md, MetadataKeyProjectID)
	hostIDStr := GetFirstOrEmpty(md, MetadataKeyHostID)
	rpcVersionStr := GetFirstOrEmpty(md, MetadataKeyRPCVersion)

	if secretToken == "" {
		return auth.AuthenticationDetails{}, nil,
			status.Errorf(codes.FailedPrecondition, "secret token is missing")
	}
	if projectIDStr == "" {
		return auth.AuthenticationDetails{}, nil,
			status.Errorf(codes.FailedPrecondition, "project ID is missing")
	}
	if hostIDStr == "" {
		return auth.AuthenticationDetails{}, nil,
			status.Errorf(codes.FailedPrecondition, "host ID is missing")
	}
	if rpcVersionStr == "" {
		return auth.AuthenticationDetails{}, nil,
			status.Errorf(codes.FailedPrecondition, "RPC version is missing")
	}

	if _, err := strconv.ParseUint(projectIDStr, 10, 32); err != nil {
		e.logger.Errorf("possible malicious client request, "+
			"converting project ID from string (%s) to uint failed: %v", projectIDStr, err)
		return auth.AuthenticationDetails{}, nil,
			status.Errorf(codes.FailedPrecondition, "invalid project ID")
	}

	if _, err := strconv.ParseUint(hostIDStr, 16, 64); err != nil {
		e.logger.Errorf("possible malicious client request, "+
			"converting host ID from string (%s) to uint failed: %v", hostIDStr, err)
		return auth.AuthenticationDetails{}, nil,
			status.Errorf(codes.FailedPrecondition, "invalid host ID")
	}

	rpcVersion64, err := strconv.ParseUint(rpcVersionStr, 10, 32)
	if err != nil {
		e.logger.Errorf("converting RPC version from string (%s) to uint failed: %v",
			rpcVersionStr, err)
		return auth.AuthenticationDetails{}, nil,
			status.Errorf(codes.FailedPrecondition, "invalid RPC version")
	}

	rpcVersion := uint32(rpcVersion64)
	if rpcVersion != rpcProtocolVersion {
		e.logger.Errorf("incompatible RPC version: %d => %d", rpcVersion, rpcProtocolVersion)

		if rpcVersion < rpcProtocolVersion {
			return auth.AuthenticationDetails{}, nil,
				status.Errorf(codes.FailedPrecondition,
					"HostAgent version is unsupported, please upgrade to the latest version")
		}

		return auth.AuthenticationDetails{}, nil,
			status.Errorf(codes.FailedPrecondition,
				"Backend is incompatible with HostAgent, please check your configuration")
	}

	return authenticator.Authenticate(ctx, headers.Bearer, secretToken)
}
