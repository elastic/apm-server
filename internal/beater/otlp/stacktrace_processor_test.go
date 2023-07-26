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
	storedParams   map[string]any
}

type TestDeobfuscator struct {
	err                  error
	deobfuscateCallCount *int
	storedParams         map[string]any
}

func TestProcessCrashEvent_deobfuscation_succeeded(t *testing.T) {
	mapBytes := []byte{'t', 'e', 's', 't'}
	fetcher := newTestFetcher(mapBytes)
	deobfuscator := newTestDeobfuscator(nil)
	processor := StacktraceProcessor{fetcher: fetcher, deobfuscator: deobfuscator}
	stacktrace := []*modelpb.StacktraceFrame{{Filename: "some name"}}
	event := createCrashAPMEvent("service-name", "1.0.0", stacktrace)

	err := processor.ProcessBatch(context.Background(), &modelpb.Batch{&event})

	assert.Nil(t, err)
	assert.Equal(t, 1, *fetcher.fetchCallCount)
	assert.Equal(t, 1, *deobfuscator.deobfuscateCallCount)
	assert.Equal(t, stacktrace, deobfuscator.storedParams["stacktrace"])
	assert.Equal(t, bytes.NewReader(mapBytes), deobfuscator.storedParams["mapFile"])
	assert.Equal(t, "service-name", fetcher.storedParams["name"])
	assert.Equal(t, "1.0.0", fetcher.storedParams["version"])
}

func TestProcessCrashEvent_deobfuscation_failed(t *testing.T) {
	mapBytes := []byte{'t', 'e', 's', 't'}
	fetcher := newTestFetcher(mapBytes)
	deobfuscator := newTestDeobfuscator(errors.New("deobfuscation error"))
	processor := StacktraceProcessor{fetcher: fetcher, deobfuscator: deobfuscator}
	stacktrace := []*modelpb.StacktraceFrame{{Filename: "some name"}}
	event := createCrashAPMEvent("service-name", "1.0.0", stacktrace)

	err := processor.ProcessBatch(context.Background(), &modelpb.Batch{&event})

	assert.NotNil(t, err)
	assert.Equal(t, 1, *fetcher.fetchCallCount)
	assert.Equal(t, 1, *deobfuscator.deobfuscateCallCount)
	assert.Equal(t, stacktrace, deobfuscator.storedParams["stacktrace"])
	assert.Equal(t, bytes.NewReader(mapBytes), deobfuscator.storedParams["mapFile"])
}

func TestProcessCrashEvent_fetching_failed(t *testing.T) {
	fetcher := newTestFetcher(nil)
	deobfuscator := newTestDeobfuscator(errors.New("deobfuscation error"))
	processor := StacktraceProcessor{fetcher: fetcher, deobfuscator: deobfuscator}
	stacktrace := []*modelpb.StacktraceFrame{{Filename: "some name"}}
	event := createCrashAPMEvent("service-name", "1.0.0", stacktrace)

	err := processor.ProcessBatch(context.Background(), &modelpb.Batch{&event})

	assert.NotNil(t, err)
	assert.Equal(t, 1, *fetcher.fetchCallCount)
	assert.Equal(t, 0, *deobfuscator.deobfuscateCallCount)
}

func TestProcessNonCrashEvent(t *testing.T) {
	mapBytes := []byte{'t', 'e', 's', 't'}
	fetcher := newTestFetcher(mapBytes)
	deobfuscator := newTestDeobfuscator(errors.New("deobfuscation error"))
	processor := StacktraceProcessor{fetcher: fetcher, deobfuscator: deobfuscator}
	event := modelpb.APMEvent{Service: createMobileService("service-name", "1.0.0")}

	err := processor.ProcessBatch(context.Background(), &modelpb.Batch{&event})

	assert.Nil(t, err)
	assert.Equal(t, 0, *fetcher.fetchCallCount)
	assert.Equal(t, 0, *deobfuscator.deobfuscateCallCount)
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
		response: response, err: err, fetchCallCount: &fetchCount, storedParams: make(map[string]any, 0),
	}
}

func createCrashAPMEvent(name, version string, stacktrace []*modelpb.StacktraceFrame) modelpb.APMEvent {
	return modelpb.APMEvent{Error: createCrashError(stacktrace), Event: createCrashEvent(), Service: createMobileService(name, version)}
}

func createMobileService(name, version string) *modelpb.Service {
	return &modelpb.Service{
		Name:    name,
		Version: version,
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
	t.storedParams["name"] = name
	t.storedParams["version"] = version
	if t.err != nil {
		return nil, t.err
	}
	return t.response, nil
}

func (d TestDeobfuscator) Deobfuscate(stacktrace *[]*modelpb.StacktraceFrame, mapFile io.Reader) error {
	*d.deobfuscateCallCount++
	d.storedParams["stacktrace"] = *stacktrace
	d.storedParams["mapFile"] = mapFile
	return d.err
}
