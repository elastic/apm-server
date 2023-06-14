package otlp

import (
	"context"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/r8"
)

type StacktraceProcessor struct {
	fetcher *r8.MapFetcher
}

// NewStacktraceProcessor returns a StacktraceProcessor.
func NewStacktraceProcessor(fetcher *r8.MapFetcher) *StacktraceProcessor {
	return &StacktraceProcessor{fetcher}
}

func (p StacktraceProcessor) ProcessBatch(ctx context.Context, batch *model.Batch) error {
	for _, event := range *batch {
		if isMobileCrash(event) {
			err := p.deobfuscate(ctx, event)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p StacktraceProcessor) deobfuscate(ctx context.Context, event model.APMEvent) error {
	mapReader, err := p.fetcher.Fetch(ctx, event.Agent.Name, event.Agent.Version)
	if err != nil {
		return err
	}
	defer mapReader.Close()

	err = r8.Deobfuscate(&event.Error.Exception.Stacktrace, mapReader)

	if err != nil {
		return err
	}
	return nil
}

func isMobileCrash(event model.APMEvent) bool {
	return event.Error != nil && event.Event.Category == "device" && event.Error.Type == "crash"
}
