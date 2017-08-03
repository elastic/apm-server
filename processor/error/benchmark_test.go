package error

import (
	"bytes"
	"testing"

	"github.com/elastic/apm-server/tests"
)

func BenchmarkEventWithFileLoading(b *testing.B) {
	processor := NewProcessor()
	for i := 0; i < b.N; i++ {
		data, _ := tests.LoadValidData("error")
		err := processor.Validate(bytes.NewReader(data))
		if err != nil {
			panic(err)
		}

		processor.Transform()
	}
}

func BenchmarkEventFileLoadingOnce(b *testing.B) {
	processor := NewProcessor()
	data, _ := tests.LoadValidData("error")
	for i := 0; i < b.N; i++ {
		processor.Validate(bytes.NewReader(data))
		processor.Transform()
	}
}
