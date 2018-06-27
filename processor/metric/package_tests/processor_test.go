package package_tests

import (
	"testing"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/processor/metric"
	"github.com/elastic/apm-server/tests"
)

func TestMetricProcessorOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessMetric", Path: "../testdata/metric/payload.json"},
		{Name: "TestProcessMetricMinimal", Path: "../testdata/metric/minimal.json"},
		{Name: "TestProcessMetricMultipleSamples", Path: "../testdata/metric/multiple-samples.json"},
		{Name: "TestProcessMetricNull", Path: "../testdata/metric/null.json"},
	}
	tests.TestProcessRequests(t, metric.NewProcessor(), config.Config{}, requestInfo, map[string]string{})
}
