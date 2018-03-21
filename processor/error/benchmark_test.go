package error

import (
	"testing"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/tests/loader"
)

func BenchmarkEventWithFileLoading(b *testing.B) {
	processor := NewProcessor(pr.Config{})
	for i := 0; i < b.N; i++ {
		data, _ := loader.LoadValidData("error")
		err := processor.Validate(data)
		if err != nil {
			panic(err)
		}

		processor.Transform(data)
	}
}

func BenchmarkEventFileLoadingOnce(b *testing.B) {
	processor := NewProcessor(pr.Config{})
	data, _ := loader.LoadValidData("error")
	for i := 0; i < b.N; i++ {
		err := processor.Validate(data)
		if err != nil {
			panic(err)
		}

		processor.Transform(data)
	}
}
