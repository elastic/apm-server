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

	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/headers"
)

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
	projectID := GetFirstOrEmpty(md, MetadataKeyProjectID)
	hostID := GetFirstOrEmpty(md, MetadataKeyHostID)

	if secretToken == "" {
		return auth.AuthenticationDetails{}, nil, status.Errorf(codes.Unauthenticated, "secret token is missing")
	}
	if projectID == "" {
		return auth.AuthenticationDetails{}, nil, status.Errorf(codes.Unauthenticated, "project ID is missing")
	}
	if hostID == "" {
		return auth.AuthenticationDetails{}, nil, status.Errorf(codes.Unauthenticated, "host ID is missing")
	}

	if _, err := strconv.Atoi(projectID); err != nil {
		e.logger.Errorf("possible malicious client request, "+
			"converting project ID from string (%s) to uint failed: %v", projectID, err)
		return auth.AuthenticationDetails{}, nil, auth.ErrAuthFailed
	}

	if _, err := strconv.ParseUint(hostID, 16, 64); err != nil {
		e.logger.Errorf("possible malicious client request, "+
			"converting host ID from string (%s) to uint failed: %v", hostID, err)
		return auth.AuthenticationDetails{}, nil, auth.ErrAuthFailed
	}

	return authenticator.Authenticate(ctx, headers.Bearer, secretToken)
}
