package error

import (
	"testing"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/tests/loader"
)

func BenchmarkEventWithFileLoading(b *testing.B) {
	processor := NewProcessor()
	for i := 0; i < b.N; i++ {
		data, _ := loader.LoadValidData("error")
		err := processor.Validate(data)
		if err != nil {
			panic(err)
		}

		payload, err := processor.Decode(data)
		if err != nil {
			b.Fatalf("Error: %v", err)
		}
		payload.Transform(config.Config{})
	}
}

func BenchmarkEventFileLoadingOnce(b *testing.B) {
	processor := NewProcessor()
	data, _ := loader.LoadValidData("error")
	for i := 0; i < b.N; i++ {
		err := processor.Validate(data)
		if err != nil {
			panic(err)
		}

		payload, err := processor.Decode(data)
		if err != nil {
			b.Fatalf("Error: %v", err)
		}
		payload.Transform(config.Config{})
	}
}
