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
	"io/ioutil"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model"
)

func BenchmarkBackendProcessor(b *testing.B) {
	cfg := config.DefaultConfig()
	processor := BackendProcessor(cfg, make(chan struct{}, cfg.MaxConcurrentDecoders))
	files, _ := filepath.Glob(filepath.FromSlash("../../testdata/intake-v2/*.ndjson"))
	benchmarkStreamProcessor(b, processor, files)
}

func BenchmarkRUMV3Processor(b *testing.B) {
	cfg := config.DefaultConfig()
	processor := RUMV3Processor(cfg, make(chan struct{}, cfg.MaxConcurrentDecoders))
	files, _ := filepath.Glob(filepath.FromSlash("../../testdata/intake-v3/rum_*.ndjson"))
	benchmarkStreamProcessor(b, processor, files)
}

func benchmarkStreamProcessor(b *testing.B, processor *Processor, files []string) {
	const batchSize = 10
	batchProcessor := nopBatchProcessor{}
	benchmark := func(b *testing.B, filename string) {
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

			var result Result
			processor.HandleStream(context.Background(), model.APMEvent{}, r, batchSize, batchProcessor, &result)
		}
	}

	for _, f := range files {
		b.Run(filepath.Base(f), func(b *testing.B) {
			benchmark(b, f)
		})
	}
}

func BenchmarkBackendProcessorParallel(b *testing.B) {
	for _, max := range []uint{0, 2, 4, 8} { // 0 is for default size.
		b.Run(fmt.Sprint(b.Name(), max), func(b *testing.B) {
			cfg := config.DefaultConfig()
			if max > 0 {
				cfg.MaxConcurrentDecoders = max
			}
			processor := BackendProcessor(cfg, make(chan struct{}, cfg.MaxConcurrentDecoders))
			files, _ := filepath.Glob(filepath.FromSlash("../../testdata/intake-v2/*.ndjson"))
			benchmarkStreamProcessorParallel(b, processor, files)
		})
	}
}

func benchmarkStreamProcessorParallel(b *testing.B, processor *Processor, files []string) {
	const batchSize = 10
	batchProcessor := nopBatchProcessor{}
	for _, f := range files {
		b.Run(filepath.Base(f), func(b *testing.B) {
			data, err := ioutil.ReadFile(f)
			if err != nil {
				b.Error(err)
			}
			b.SetBytes(int64(len(data)))
			b.RunParallel(func(p *testing.PB) {
				r := bytes.NewReader(data)
				for p.Next() {
					var result Result
					processor.HandleStream(context.Background(), model.APMEvent{}, r, batchSize, batchProcessor, &result)
					r.Seek(0, io.SeekStart)
				}
			})
		})
	}
}

func BenchmarkBackendProcessorParallelSemSize(b *testing.B) {
	for _, semSize := range []uint{1, 2, 4, 8} {
		b.Run(fmt.Sprint(semSize), func(b *testing.B) {
			benchmarkBackendProcessorParallelSemSize(b, semSize)
		})
	}
}

func benchmarkBackendProcessorParallelSemSize(b *testing.B, size uint) {
	cfg := config.DefaultConfig()
	cfg.MaxConcurrentDecoders = size
	processor := BackendProcessor(cfg, make(chan struct{}, cfg.MaxConcurrentDecoders))
	file := "../../testdata/intake-v2/heavy.ndjson"
	const batchSize = 10
	batchProcessor := nopBatchProcessor{}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		b.Error(err)
	}
	b.SetBytes(int64(len(data)))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	b.ResetTimer()
	b.RunParallel(func(p *testing.PB) {
		r := bytes.NewReader(data)
		for p.Next() {
			var result Result
			processor.HandleStream(ctx, model.APMEvent{}, r, batchSize, batchProcessor, &result)
			r.Seek(0, io.SeekStart)
		}
	})
}

type mutexBatchProcessor struct {
	rwm sync.RWMutex
	mu  sync.Mutex
}

func (p *mutexBatchProcessor) ProcessBatch(_ context.Context, b *model.Batch) error {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	// Simulate modelindexer
	for range *b {
		p.mu.Lock()
		p.mu.Unlock()
	}
	return nil
}
