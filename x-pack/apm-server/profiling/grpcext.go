// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package profiling

import (
	"context"
	"strconv"

	"google.golang.org/grpc/metadata"
)

const (
	MetadataKeyProjectID   = "projectID"
	MetadataKeySecretToken = "secretToken"
	MetadataKeyHostID      = "hostID"
)

func GetFirstOrEmpty(md metadata.MD, key string) string {
	v := md.Get(key)
	if len(v) > 0 {
		return v[0]
	}
	return ""
}

func GetProjectID(ctx context.Context) uint32 {
	// Metadata and host ID have been validated in auth interceptor,
	// no need to error check here.
	md, _ := metadata.FromIncomingContext(ctx)
	projectIDs := GetFirstOrEmpty(md, MetadataKeyProjectID)
	projectID64, _ := strconv.Atoi(projectIDs)
	return uint32(projectID64)
}

func GetHostID(ctx context.Context) uint64 {
	// Metadata and host ID have been validated in auth interceptor,
	// no need to error check here.
	md, _ := metadata.FromIncomingContext(ctx)
	hostIDs := GetFirstOrEmpty(md, MetadataKeyHostID)
	hostID, _ := strconv.ParseUint(hostIDs, 16, 64)
	return hostID
}
