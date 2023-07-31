package otlp

import (
	"bytes"
	"context"
	"io"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/r8"
)

type MapFetcher interface {
	Fetch(ctx context.Context, name, version string) ([]byte, error)
}

type CrashDeobfuscator interface {
	Deobfuscate(stacktrace *[]*modelpb.StacktraceFrame, mapFile io.Reader) error
}

type StacktraceProcessor struct {
	fetcher      MapFetcher
	deobfuscator CrashDeobfuscator
}

// NewStacktraceProcessor returns a StacktraceProcessor.
func NewStacktraceProcessor(fetcher MapFetcher, deobfuscator CrashDeobfuscator) *StacktraceProcessor {
	return &StacktraceProcessor{fetcher, deobfuscator}
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
	mapBytes, err := p.fetcher.Fetch(ctx, event.Service.Name, event.Service.Version)
	if err != nil {
		return err
	}

	err = p.deobfuscator.Deobfuscate(&event.Error.Exception.Stacktrace, bytes.NewReader(mapBytes))

	if err != nil {
		return err
	}
	return nil
}

func isMobileCrash(event *modelpb.APMEvent) bool {
	return event.Error != nil && event.Event.Category == "device" && event.Error.Type == "crash"
}

type crashDeobfuscatorFunc func(stacktrace *[]*modelpb.StacktraceFrame, mapFile io.Reader) error

func (d crashDeobfuscatorFunc) Deobfuscate(stacktrace *[]*modelpb.StacktraceFrame, mapFile io.Reader) error {
	return d(stacktrace, mapFile)
}
