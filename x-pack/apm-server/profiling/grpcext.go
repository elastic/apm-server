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
	MetadataKeyProjectID     = "projectID"
	MetadataKeySecretToken   = "secretToken"
	MetadataKeyVersion       = "version"
	MetadataKeyRevision      = "revision"
	MetadataKeyIPAddress     = "ipAddress"
	MetadataKeyHostname      = "hostname"
	MetadataKeyKernelVersion = "kernelVersion"
	MetadataKeyHostID        = "hostID"
	MetadataKeyRPCVersion    = "rpcVersion"
	// Tags will be auto base64 encoded/decoded
	MetadataKeyTags = "tags-bin"
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
	projectIDStr := GetFirstOrEmpty(md, MetadataKeyProjectID)
	projectID64, _ := strconv.ParseUint(projectIDStr, 10, 32)
	return uint32(projectID64)
}

func GetHostID(ctx context.Context) uint64 {
	// Metadata and host ID have been validated in auth interceptor,
	// no need to error check here.
	md, _ := metadata.FromIncomingContext(ctx)
	hostIDStr := GetFirstOrEmpty(md, MetadataKeyHostID)
	hostID, _ := strconv.ParseUint(hostIDStr, 16, 64)
	return hostID
}

func GetSecretToken(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	return GetFirstOrEmpty(md, MetadataKeySecretToken)
}

func GetVersion(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	return GetFirstOrEmpty(md, MetadataKeyVersion)
}

func GetRevision(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	return GetFirstOrEmpty(md, MetadataKeyRevision)
}

func GetIPAddress(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	return GetFirstOrEmpty(md, MetadataKeyIPAddress)
}

func GetHostname(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	return GetFirstOrEmpty(md, MetadataKeyHostname)
}

func GetKernelVersion(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	return GetFirstOrEmpty(md, MetadataKeyKernelVersion)
}

func GetTags(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	return GetFirstOrEmpty(md, MetadataKeyTags)
}
