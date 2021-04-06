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

package stream

import (
	"bytes"
	"context"
	"io/ioutil"
	"math"
	"path/filepath"
	"testing"

	"golang.org/x/time/rate"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model"
)

func BenchmarkBackendProcessor(b *testing.B) {
	processor := BackendProcessor(config.DefaultConfig())
	files, _ := filepath.Glob(filepath.FromSlash("../../testdata/intake-v2/*.ndjson"))
	benchmarkStreamProcessor(b, processor, files)
}

func BenchmarkRUMV3Processor(b *testing.B) {
	processor := RUMV3Processor(config.DefaultConfig())
	files, _ := filepath.Glob(filepath.FromSlash("../../testdata/intake-v3/rum_*.ndjson"))
	benchmarkStreamProcessor(b, processor, files)
}

func benchmarkStreamProcessor(b *testing.B, processor *Processor, files []string) {
	batchProcessor := nopBatchProcessor{}

	//ensure to not hit rate limit as blocking wait would be measured otherwise
	rl := rate.NewLimiter(rate.Limit(math.MaxFloat64-1), math.MaxInt32)

	benchmark := func(filename string, rl *rate.Limiter) func(b *testing.B) {
		return func(b *testing.B) {
			data, err := ioutil.ReadFile(filename)
			if err != nil {
				b.Error(err)
			}
			r := bytes.NewReader(data)
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				r.Reset(data)
				b.StartTimer()
				processor.HandleStream(context.Background(), rl, &model.Metadata{}, r, batchProcessor)
			}
		}
	}

	for _, f := range files {
		b.Run(filepath.Base(f), func(b *testing.B) {
			b.Run("NoRateLimit", benchmark(f, nil))
			b.Run("WithRateLimit", benchmark(f, rl))
		})
	}
}
