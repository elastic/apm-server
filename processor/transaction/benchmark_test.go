package transaction

import (
	"testing"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/tests/loader"
)

func BenchmarkWithFileLoading(b *testing.B) {
	processor := NewProcessor()
	for i := 0; i < b.N; i++ {
		data, _ := loader.LoadValidData("transaction")
		err := processor.Validate(data)
		if err != nil {
			b.Fatalf("Error: %v", err)
		}
		payload, err := processor.Decode(data)
		if err != nil {
			b.Fatalf("Error: %v", err)
		}
		payload.Transform(config.Config{})
	}
}

func BenchmarkTransactionFileLoadingOnce(b *testing.B) {
	processor := NewProcessor()
	data, _ := loader.LoadValidData("transaction")
	for i := 0; i < b.N; i++ {
		err := processor.Validate(data)
		if err != nil {
			payload, err := processor.Decode(data)
			if err != nil {
				b.Fatalf("Error: %v", err)
			}
			payload.Transform(config.Config{})
			b.Fatalf("Error: %v", err)
		}
	}
}
