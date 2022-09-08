// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package profiling

import (
	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-server/internal/beater/request"
)

var (
	monitoringRegistry = monitoring.Default.NewRegistry("apm-server.profiling.grpc.collect")
	monitoringMap      = request.MonitoringMapForRegistry(monitoringRegistry,
		append(request.DefaultResultIDs,
			request.IDResponseErrorsRateLimit,
			request.IDResponseErrorsTimeout,
			request.IDResponseErrorsUnauthorized,
		),
	)
)

// RequestMetrics returns the request metrics registry for the profiling service.
func (*ElasticCollector) RequestMetrics(fullMethodName string) map[request.ResultID]*monitoring.Int {
	return monitoringMap
}
