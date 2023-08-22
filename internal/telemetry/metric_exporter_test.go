// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package telemetry

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/elastic/apm-data/model/modelpb"
)

func TestMetricExporter(t *testing.T) {
	p := modelpb.ProcessBatchFunc(func(ctx context.Context, b *modelpb.Batch) error {
		fmt.Fprintf(os.Stdout, "%#v\n", b)
		return nil
	})
	e := NewMetricExporter(p)

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(e)),
	)
	meter := provider.Meter("test")

	counter, err := meter.Float64Counter("foo")
	assert.NoError(t, err)
	counter.Add(context.Background(), 5)

	provider.Shutdown(context.Background())
}
