package intake

import (
	"github.com/elastic/apm-server/beater/request"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
)

func OtlpTracesHandler(receivers *otlpreceiver.HTTPReceivers) request.Handler {
	return func(c *request.Context) {
		otlpreceiver.HandleHTTPTraces(c.W, c.Request, receivers.TracesR)
	}
}

func OtlpMetricsHandler(receivers *otlpreceiver.HTTPReceivers) request.Handler {
	return func(c *request.Context) {
		otlpreceiver.HandleHTTPMetrics(c.W, c.Request, receivers.MetricsR)
	}
}

func OtlpLogsHandler(receivers *otlpreceiver.HTTPReceivers) request.Handler {
	return func(c *request.Context) {
		otlpreceiver.HandleHTTPLogs(c.W, c.Request, receivers.LogsR)
	}
}
