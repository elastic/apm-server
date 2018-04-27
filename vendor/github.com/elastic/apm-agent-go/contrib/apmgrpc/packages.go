package apmgrpc

import "github.com/elastic/apm-agent-go/stacktrace"

func init() {
	stacktrace.RegisterLibraryPackage(
		"google.golang.org/grpc",
		"github.com/grpc-ecosystem",
	)
}
