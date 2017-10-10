package error

import (
	"testing"

	"github.com/elastic/apm-server/tests"
)

func BenchmarkEventWithFileLoading(b *testing.B) {
	processor := NewBackendProcessor()
	for i := 0; i < b.N; i++ {
		data, _ := tests.LoadValidData("error")
		err := processor.Validate(data)
		if err != nil {
			panic(err)
		}

		processor.Transform(data)
	}
}

func BenchmarkEventFileLoadingOnce(b *testing.B) {
	processor := NewBackendProcessor()
	data, _ := tests.LoadValidData("error")
	for i := 0; i < b.N; i++ {
		err := processor.Validate(data)
		if err != nil {
			panic(err)
		}

		processor.Transform(data)
	}
}
