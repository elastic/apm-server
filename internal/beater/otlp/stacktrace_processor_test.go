package otlp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/stretchr/testify/assert"
)

type TestFetcher struct {
	response       []byte
	err            error
	fetchCallCount *int
}

type TestDeobfuscator struct {
	err                  error
	deobfuscateCallCount *int
	storedParams         map[string]any
}

func TestProcessCrashEvent(t *testing.T) {
	mapBytes := []byte{'t', 'e', 's', 't'}
	fetcher := newTestFetcher(mapBytes)
	deobfuscator := newTestDeobfuscator(nil)
	processor := StacktraceProcessor{fetcher: fetcher, deobfuscator: deobfuscator}
	stacktrace := []*modelpb.StacktraceFrame{{Filename: "some name"}}
	event := createCrashAPMEvent(stacktrace)

	err := processor.ProcessBatch(context.Background(), &modelpb.Batch{&event})

	assert.Nil(t, err)
	assert.Equal(t, 1, *fetcher.fetchCallCount)
	assert.Equal(t, 1, *deobfuscator.deobfuscateCallCount)
	assert.Equal(t, stacktrace, deobfuscator.storedParams["stacktrace"])
	assert.Equal(t, bytes.NewReader(mapBytes), deobfuscator.storedParams["mapFile"])
}

func newTestDeobfuscator(err error) TestDeobfuscator {
	deobfuscateCallCount := 0
	return TestDeobfuscator{err: err, deobfuscateCallCount: &deobfuscateCallCount, storedParams: make(map[string]any, 0)}
}

func newTestFetcher(response []byte) TestFetcher {
	fetchCount := 0
	var err error
	if response == nil {
		err = errors.New("no map found")
	} else {
		err = nil
	}
	return TestFetcher{
		response: response, err: err, fetchCallCount: &fetchCount,
	}
}

func createCrashAPMEvent(stacktrace []*modelpb.StacktraceFrame) modelpb.APMEvent {
	return modelpb.APMEvent{Error: createCrashError(stacktrace), Event: createCrashEvent(), Service: createMobileService()}
}

func createMobileService() *modelpb.Service {
	return &modelpb.Service{
		Name:    "service-name",
		Version: "1.0.0",
	}
}

func createCrashError(stacktrace []*modelpb.StacktraceFrame) *modelpb.Error {
	return &modelpb.Error{
		Type:      "crash",
		Exception: &modelpb.Exception{Stacktrace: stacktrace},
	}
}

func createCrashEvent() *modelpb.Event {
	return &modelpb.Event{
		Category: "device",
	}
}

func (t TestFetcher) Fetch(ctx context.Context, name, version string) ([]byte, error) {
	*t.fetchCallCount++
	if t.err != nil {
		return nil, t.err
	}
	return t.response, nil
}

func (d TestDeobfuscator) Deobfuscate(stacktrace *[]*modelpb.StacktraceFrame, mapFile io.Reader) error {
	*d.deobfuscateCallCount++
	d.storedParams["stacktrace"] = *stacktrace
	d.storedParams["mapFile"] = mapFile
	return nil
}
