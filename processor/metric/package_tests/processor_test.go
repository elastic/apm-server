package package_tests

import (
	"testing"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/processor/metric"
	"github.com/elastic/apm-server/tests"
)

var (
	requestInfo = []tests.RequestInfo{
		{Name: "TestProcessMetric", Path: "../testdata/metric/payload.json"},
		{Name: "TestProcessMetricMinimal", Path: "../testdata/metric/minimal.json"},
		{Name: "TestProcessMetricMultipleSamples", Path: "../testdata/metric/multiple-samples.json"},
		{Name: "TestProcessMetricNull", Path: "../testdata/metric/null.json"},
	}
)

func TestMetricProcessorOK(t *testing.T) {
	tests.TestProcessRequests(t, metric.NewProcessor(), config.Config{}, requestInfo, map[string]string{})
}

func BenchmarkProcessor(b *testing.B) {
	tests.BenchmarkProcessRequests(b, metric.NewProcessor(), config.Config{}, requestInfo)
}
