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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-data/model"
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

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			b.Error(err)
		}
		r := bytes.NewReader(data)

		b.Run(filepath.Base(f), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				r.Seek(0, io.SeekStart)

				var result Result
				processor.HandleStream(context.Background(), false, model.APMEvent{}, r, batchSize, batchProcessor, &result)
			}
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
					processor.HandleStream(context.Background(), false, model.APMEvent{}, r, batchSize, batchProcessor, &result)
					r.Seek(0, io.SeekStart)
				}
			})
		})
	}
}

func BenchmarkBackendProcessorAsync(b *testing.B) {
	processor := BackendProcessor(Config{
		MaxEventSize: 300 * 1024, // 300 kb
		Semaphore:    make(chan struct{}, 200),
	})
	files, _ := filepath.Glob(filepath.FromSlash("../../../testdata/intake-v2/heavy.ndjson"))
	benchmarkStreamProcessorAsync(b, processor, files)
}

func benchmarkStreamProcessorAsync(b *testing.B, processor *Processor, files []string) {
	const batchSize = 10

	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(b, err)
		r := bytes.NewReader(data)

		events := -1 // Exclude metadata
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			events++
		}
		require.NoError(b, scanner.Err())

		batches := events / batchSize
		if events >= 10 && events%batchSize > 0 {
			batches++
		}
		if batches == 0 {
			batches++
		}

		r.Seek(0, io.SeekStart)
		// allow the channel to immediately process all the batches
		batchProcessor := &accountProcessor{batch: make(chan *model.Batch, batches)}

		b.Run(filepath.Base(f), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				r.Seek(0, io.SeekStart)

				var result Result
				processor.HandleStream(context.Background(), true, model.APMEvent{}, r, batchSize, batchProcessor, &result)
				// drain the batches
				for i := 0; i < batches; i++ {
					<-batchProcessor.batch
				}
			}
		})
	}
}

func BenchmarkReadBatch(b *testing.B) {
	const batchSize = 10
	processor := BackendProcessor(Config{
		MaxEventSize: 300 * 1024, // 300 kb
		Semaphore:    make(chan struct{}, 200),
	})

	files, _ := filepath.Glob(filepath.FromSlash("../../../testdata/intake-v2/*.ndjson"))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			b.Fatal(err)
		}

		r := bytes.NewReader(data)

		// We cannot rely on the sync.Pool for consistent
		// behaviour so we get the stream reader once and
		// reset it on every loop.
		sr := processor.getStreamReader(r)

		// Allocate a slice big enough to fit all
		// the events so that we can avoid the
		// overhead of resizing while reading.
		batch := make(model.Batch, 0, 17)

		b.Run(filepath.Base(f), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))

			for i := 0; i < b.N; i++ {
				baseEvent := model.APMEvent{}
				processor.readMetadata(sr, &baseEvent)

				var readErr error
				for readErr != io.EOF {
					// Reuse the slice
					batch = batch[:0]
					_, readErr = processor.readBatch(context.Background(), baseEvent, batchSize, &batch, sr, &Result{})
				}

				r.Seek(0, io.SeekStart)
				sr.Reset(r)
			}
		})
	}
}
