package otlp

import (
	"context"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/otel"
	"github.com/elastic/beats/v7/libbeat/monitoring"
	"github.com/pkg/errors"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
)

var (
	httpMetricsRegistry      = monitoring.Default.NewRegistry("apm-server.otlp.http.metrics")
	HttpMetricsMonitoringMap = request.MonitoringMapForRegistry(httpMetricsRegistry, monitoringKeys)
	httpTracesRegistry       = monitoring.Default.NewRegistry("apm-server.otlp.http.traces")
	HttpTracesMonitoringMap  = request.MonitoringMapForRegistry(httpTracesRegistry, monitoringKeys)
	httpLogsRegistry         = monitoring.Default.NewRegistry("apm-server.otlp.http.logs")
	HttpLogsMonitoringMap    = request.MonitoringMapForRegistry(httpLogsRegistry, monitoringKeys)
)

func init() {
	monitoring.NewFunc(httpMetricsRegistry, "httpConsumer", collectMetricsMonitoring, monitoring.Report)
}

func NewHTTPReceivers(processor model.BatchProcessor) (*otlpreceiver.HTTPReceivers, error) {
	consumer := &otel.Consumer{Processor: processor}
	setCurrentMonitoredConsumer(consumer)

	tracesR, err := otlpreceiver.NewHTTPTraceReceiver(context.Background(), consumer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OTLP trace receiver")
	}
	metricsR, err := otlpreceiver.NewHTTPMetricsReceiver(context.Background(), consumer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OTLP metrics receiver")
	}
	logsR, err := otlpreceiver.NewHTTPLogsReceiver(context.Background(), consumer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OTLP logs receiver")
	}
	return &otlpreceiver.HTTPReceivers{
		TracesR:  tracesR,
		MetricsR: metricsR,
		LogsR:    logsR,
	}, nil
}
