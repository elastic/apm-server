package otlp

import (
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/processor/otel"
	"github.com/elastic/beats/v7/libbeat/monitoring"
	"sync"
)

var (
	monitoringKeys = append(request.DefaultResultIDs,
		request.IDResponseErrorsRateLimit,
		request.IDResponseErrorsTimeout,
		request.IDResponseErrorsUnauthorized,
	)
	currentMonitoredConsumerMu sync.RWMutex
	currentMonitoredConsumer   *otel.Consumer
)

const (
	metricsFullMethod = "/opentelemetry.proto.collector.metrics.v1.MetricsService/Export"
	tracesFullMethod  = "/opentelemetry.proto.collector.trace.v1.TraceService/Export"
	logsFullMethod    = "/opentelemetry.proto.collector.logs.v1.LogsService/Export"
)

func collectMetricsMonitoring(mode monitoring.Mode, V monitoring.Visitor) {
	V.OnRegistryStart()
	defer V.OnRegistryFinished()

	currentMonitoredConsumerMu.RLock()
	c := currentMonitoredConsumer
	currentMonitoredConsumerMu.RUnlock()
	if c == nil {
		return
	}

	stats := c.Stats()
	monitoring.ReportInt(V, "unsupported_dropped", stats.UnsupportedMetricsDropped)
}

func setCurrentMonitoredConsumer(c *otel.Consumer) {
	currentMonitoredConsumerMu.Lock()
	defer currentMonitoredConsumerMu.Unlock()
	currentMonitoredConsumer = c
}
