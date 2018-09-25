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
	"errors"
	"io/ioutil"
	"math"
	"path/filepath"
	"runtime"
	"testing"

	r "golang.org/x/time/rate"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests/loader"
)

func BenchmarkStreamProcessor(b *testing.B) {
	report := func(ctx context.Context, p publish.PendingReq) error {
		return nil
	}
	dir := "../testdata/intake-v2"
	_, cwd, _, ok := runtime.Caller(0)
	if !ok {
		b.Error(errors.New("Could not determine test dir"))
	}
	files, err := ioutil.ReadDir(filepath.Join(cwd, "../..", dir))
	if err != nil {
		b.Error(err)
	}
	//ensure to not hit rate limit as blocking wait would be measured otherwise
	ctx := ContextWithRateLimiter(context.Background(), r.NewLimiter(r.Limit(math.MaxFloat64-1), math.MaxInt32))
	sp := &StreamProcessor{}
	for _, f := range files {
		b.Run(f.Name(), func(b *testing.B) {
			data, err := loader.LoadDataAsBytes(filepath.Join(dir, f.Name()))
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
				reader := decoder.NewNDJSONStreamReader(r, 100000)
				b.StartTimer()
				sp.HandleStream(ctx, map[string]interface{}{}, reader, report)
			}
		})
	}
}
