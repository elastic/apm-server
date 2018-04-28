package model_test

import (
	"testing"

	"github.com/elastic/apm-agent-go/internal/fastjson"
)

func BenchmarkMarshalTransactionPayloadStdlib(b *testing.B) {
	p := fakeTransactionsPayload(1000)
	b.ResetTimer()

	var w fastjson.Writer
	for i := 0; i < b.N; i++ {
		p.MarshalFastJSON(&w)
		w.Reset()
	}
}
