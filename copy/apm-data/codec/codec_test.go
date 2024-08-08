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

package codec

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

func TestMetrics(t *testing.T) {
	rdr := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr))
	meter := mp.Meter("test")

	em, err := meter.Int64Counter("encoded")
	require.NoError(t, err)

	var codec testCodec
	encoder := RecordEncodedBytes(codec, em)
	b, err := encoder.Encode(map[string]any{"a": "b"})
	require.NoError(t, err)

	var rm metricdata.ResourceMetrics
	assert.NoError(t, rdr.Collect(context.Background(), &rm))

	metric := rm.ScopeMetrics[0].Metrics[0]

	metricdatatest.AssertEqual(t, metricdata.Metrics{
		Name: "encoded",
		Data: metricdata.Sum[int64]{
			Temporality: metricdata.CumulativeTemporality,
			IsMonotonic: true,
			DataPoints: []metricdata.DataPoint[int64]{
				{Value: 5},
			},
		},
	}, metric, metricdatatest.IgnoreTimestamp())

	dm, err := meter.Int64Counter("decoded")
	require.NoError(t, err)

	decoder := RecordDecodedBytes(codec, dm)
	out := make(map[string]any)
	require.NoError(t, decoder.Decode(b, &out))
	assert.NoError(t, rdr.Collect(context.Background(), &rm))

	// Decoded metric
	metric = rm.ScopeMetrics[0].Metrics[1]
	metricdatatest.AssertEqual(t, metricdata.Metrics{
		Name: "decoded",
		Data: metricdata.Sum[int64]{
			Temporality: metricdata.CumulativeTemporality,
			IsMonotonic: true,
			DataPoints: []metricdata.DataPoint[int64]{
				{Value: 5},
			},
		},
	}, metric, metricdatatest.IgnoreTimestamp())
}

func BenchmarkEncode(b *testing.B) {
	ev := fullEvent(b)
	codecs := map[string]Codec{
		"json":    JSON{},
		"vtproto": VTProto{},
	}
	for name, c := range codecs {
		b.Run("format="+name, func(b *testing.B) {
			var output atomic.Int64
			b.RunParallel(func(p *testing.PB) {
				var localOutput int64
				for p.Next() {
					by, _ := c.Encode(ev)
					localOutput += int64(len(by))
				}
				output.Add(localOutput)
			})
			bytePerOp := float64(output.Load()) / float64(b.N)
			b.ReportMetric(bytePerOp, "bytes/op")
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	ev := fullEvent(b)
	codecs := map[string]Codec{
		"json":    JSON{},
		"vtproto": VTProto{},
	}
	for name, c := range codecs {
		encoded, _ := c.Encode(ev)
		b.Run("format="+name, func(b *testing.B) {
			b.RunParallel(func(p *testing.PB) {
				for p.Next() {
					e := &modelpb.APMEvent{}
					c.Decode(encoded, e)
				}
			})
		})

		bytePerOp := float64(len(encoded))
		b.ReportMetric(bytePerOp, "bytes/op")
	}
}

type testCodec struct {
}

func (c testCodec) Encode(any) ([]byte, error) {
	return []byte("dummy"), nil
}

func (c testCodec) Decode([]byte, any) error {
	return nil
}
