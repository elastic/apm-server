package otlp

import (
	"bytes"
	"context"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/r8"
)

type StacktraceProcessor struct {
	fetcher *r8.MapFetcher
}

// NewStacktraceProcessor returns a StacktraceProcessor.
func NewStacktraceProcessor(fetcher *r8.MapFetcher) *StacktraceProcessor {
	return &StacktraceProcessor{fetcher}
}

func (p StacktraceProcessor) ProcessBatch(ctx context.Context, batch *modelpb.Batch) error {
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

func (p StacktraceProcessor) deobfuscate(ctx context.Context, event *modelpb.APMEvent) error {
	mapBytes, err := p.fetcher.Fetch(ctx, event.Agent.Name, event.Agent.Version)
	if err != nil {
		return err
	}

	err = r8.Deobfuscate(&event.Error.Exception.Stacktrace, bytes.NewReader(mapBytes))

	if err != nil {
		return err
	}
	return nil
}

func isMobileCrash(event *modelpb.APMEvent) bool {
	return event.Error != nil && event.Event.Category == "device" && event.Error.Type == "crash"
}
