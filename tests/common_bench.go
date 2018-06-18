package tests

import (
	"testing"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/tests/loader"
)

func benchmarkValidate(b *testing.B, p processor.Processor, requestInfo RequestInfo) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		data, err := loader.LoadData(requestInfo.Path)
		if err != nil {
			b.Error(err)
		}
		b.StartTimer()
		if err := p.Validate(data); err != nil {
			b.Error(err)
		}
	}
}

func benchmarkDecode(b *testing.B, p processor.Processor, requestInfo RequestInfo) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		data, err := loader.LoadData(requestInfo.Path)
		if err != nil {
			b.Error(err)
		}
		b.StartTimer()
		if _, err := p.Decode(data); err != nil {
			b.Error(err)
		}
	}
}

func benchmarkTransform(b *testing.B, p processor.Processor, config config.Config, requestInfo RequestInfo) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		data, err := loader.LoadData(requestInfo.Path)
		if err != nil {
			b.Error(err)
		}
		if payload, err := p.Decode(data); err != nil {
			b.Error(err)
		} else {
			b.StartTimer()
			payload.Transform(config)
		}
	}
}

func benchmarkProcessRequest(b *testing.B, p processor.Processor, config config.Config, requestInfo RequestInfo) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		data, err := loader.LoadData(requestInfo.Path)
		if err != nil {
			b.Error(err)
		}
		b.StartTimer()
		if err := p.Validate(data); err != nil {
			b.Error(err)
		}
		if payload, err := p.Decode(data); err != nil {
			b.Error(err)
		} else {
			payload.Transform(config)
		}
	}
}

func BenchmarkProcessRequests(b *testing.B, p processor.Processor, config config.Config, requestInfo []RequestInfo) {
	for _, info := range requestInfo {
		validate := func(b *testing.B) {
			benchmarkValidate(b, p, info)
		}
		decode := func(b *testing.B) {
			benchmarkDecode(b, p, info)
		}
		transform := func(b *testing.B) {
			benchmarkTransform(b, p, config, info)
		}
		processRequest := func(b *testing.B) {
			benchmarkProcessRequest(b, p, config, info)
		}
		b.Run(info.Name+"Validate", validate)
		b.Run(info.Name+"Decode", decode)
		b.Run(info.Name+"Transform", transform)
		b.Run(info.Name+"ProcessRequest", processRequest)
	}
}
