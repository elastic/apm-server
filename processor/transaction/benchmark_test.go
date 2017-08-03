package transaction

import (
	"bytes"
	"testing"

	"github.com/elastic/apm-server/tests"
)

func BenchmarkWithFileLoading(b *testing.B) {
	processor := NewProcessor()
	for i := 0; i < b.N; i++ {
		data, _ := tests.LoadValidData("transaction")
		processor.Validate(bytes.NewReader(data))
		processor.Transform()
	}
}

func BenchmarkTransactionFileLoadingOnce(b *testing.B) {
	processor := NewProcessor()
	data, _ := tests.LoadValidData("transaction")
	for i := 0; i < b.N; i++ {
		processor.Validate(bytes.NewReader(data))
		processor.Transform()
	}
}
