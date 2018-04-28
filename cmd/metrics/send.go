package main

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/contrib/apmprometheus"
)

func main() {
	elasticapm.DefaultTracer.AddMetricsGatherer(apmprometheus.Wrap(prometheus.DefaultGatherer))
	elasticapm.DefaultTracer.SendMetrics(nil)
}
