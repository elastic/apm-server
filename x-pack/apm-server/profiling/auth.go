// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package profiling

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/headers"
)

// AuthenticateUnaryCall implements the interceptors.UnaryAuthenticator
// interface, extracting the secret token supplied by the Host Agent,
// which we will treat the same as an APM secret token.
func (*ElasticCollector) AuthenticateUnaryCall(
	ctx context.Context,
	req interface{},
	fullMethod string,
	authenticator *auth.Authenticator,
) (auth.AuthenticationDetails, auth.Authorizer, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	secretToken := GetFirstOrEmpty(md, MetadataKeySecretToken)
	return authenticator.Authenticate(ctx, headers.Bearer, secretToken)
}
