// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlptext

import (
	"go.opentelemetry.io/collector/model/pdata"
)

// NewTextMetricsMarshaler returns a serializer.MetricsMarshaler to encode to OTLP text bytes.
func NewTextMetricsMarshaler() pdata.MetricsMarshaler {
	return textMetricsMarshaler{}
}

type textMetricsMarshaler struct{}

// MarshalMetrics pdata.Metrics to OTLP text.
func (textMetricsMarshaler) MarshalMetrics(md pdata.Metrics) ([]byte, error) {
	buf := dataBuffer{}
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		buf.logEntry("ResourceMetrics #%d", i)
		rm := rms.At(i)
		buf.logAttributeMap("Resource labels", rm.Resource().Attributes())
		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			buf.logEntry("InstrumentationLibraryMetrics #%d", j)
			ilm := ilms.At(j)
			buf.logInstrumentationLibrary(ilm.InstrumentationLibrary())
			metrics := ilm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				buf.logEntry("Metric #%d", k)
				metric := metrics.At(k)
				buf.logMetricDescriptor(metric)
				buf.logMetricDataPoints(metric)
			}
		}
	}

	return buf.buf.Bytes(), nil
}
