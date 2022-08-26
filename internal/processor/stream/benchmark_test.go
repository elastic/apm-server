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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/elastic/apm-server/internal/model"
)

func BenchmarkBackendProcessor(b *testing.B) {
	processor := BackendProcessor(Config{
		MaxEventSize: 300 * 1024, // 300 kb
		Semaphore:    make(chan struct{}, 200),
	})
	files, _ := filepath.Glob(filepath.FromSlash("../../../testdata/intake-v2/*.ndjson"))
	benchmarkStreamProcessor(b, processor, files)
}

func BenchmarkRUMV3Processor(b *testing.B) {
	processor := BackendProcessor(Config{
		MaxEventSize: 300 * 1024, // 300 kb
		Semaphore:    make(chan struct{}, 200),
	})
	files, _ := filepath.Glob(filepath.FromSlash("../../../testdata/intake-v3/rum_*.ndjson"))
	benchmarkStreamProcessor(b, processor, files)
}

func benchmarkStreamProcessor(b *testing.B, processor *Processor, files []string) {
	const batchSize = 10
	batchProcessor := nopBatchProcessor{}
	benchmark := func(b *testing.B, filename string) {
		data, err := os.ReadFile(filename)
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

			var result Result
			processor.HandleStream(context.Background(), false, model.APMEvent{}, io.NopCloser(r), batchSize, batchProcessor, &result)
		}
	}

	for _, f := range files {
		b.Run(filepath.Base(f), func(b *testing.B) {
			benchmark(b, f)
		})
	}
}

func BenchmarkBackendProcessorParallel(b *testing.B) {
	for _, max := range []uint{2, 4, 8, 200} {
		b.Run(fmt.Sprint(b.Name(), max), func(b *testing.B) {
			processor := BackendProcessor(Config{
				MaxEventSize: 300 * 1024, // 300 kb
				Semaphore:    make(chan struct{}, max),
			})
			files, _ := filepath.Glob(filepath.FromSlash("../../../testdata/intake-v2/*.ndjson"))
			benchmarkStreamProcessorParallel(b, processor, files)
		})
	}
}

func benchmarkStreamProcessorParallel(b *testing.B, processor *Processor, files []string) {
	const batchSize = 10
	batchProcessor := nopBatchProcessor{}
	for _, f := range files {
		b.Run(filepath.Base(f), func(b *testing.B) {
			data, err := os.ReadFile(f)
			if err != nil {
				b.Error(err)
			}
			b.SetBytes(int64(len(data)))
			b.RunParallel(func(p *testing.PB) {
				r := bytes.NewReader(data)
				for p.Next() {
					var result Result
					processor.HandleStream(context.Background(), false, model.APMEvent{}, io.NopCloser(r), batchSize, batchProcessor, &result)
					r.Seek(0, io.SeekStart)
				}
			})
		})
	}
}
