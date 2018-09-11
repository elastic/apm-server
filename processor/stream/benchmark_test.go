package stream

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests/loader"
)

func BenchmarkStreamProcessor(b *testing.B) {
	report := func(ctx context.Context, p publish.PendingReq) error {
		return nil
	}
	b.ResetTimer()

	dir := "../testdata/intake-v2"
	_, cwd, _, ok := runtime.Caller(0)
	if !ok {
		b.Error(errors.New("Could not determine test dir"))
	}
	files, err := ioutil.ReadDir(filepath.Join(cwd, "../..", dir))
	if err != nil {
		b.Error(err)
	}
	for _, f := range files {
		b.Run(f.Name(), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				data, err := loader.LoadDataAsBytes(filepath.Join(dir, f.Name()))
				if err != nil {
					b.Error(err)
				}
				bodyReader := bytes.NewBuffer(data)
				reader := decoder.NewNDJSONStreamReader(bodyReader)
				ctx := context.Background()
				b.StartTimer()
				(&StreamProcessor{}).HandleStream(ctx, map[string]interface{}{}, reader, report)
			}
		})
	}
}
