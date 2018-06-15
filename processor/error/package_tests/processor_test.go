package package_tests

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	s "github.com/go-sourcemap/sourcemap"

	"github.com/elastic/apm-server/config"
	er "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/tests"
)

var (
	backendRequestInfo = []tests.RequestInfo{
		{Name: "TestProcessErrorMinimalService", Path: "data/valid/error/minimal_service.json"},
		{Name: "TestProcessErrorMinimalProcess", Path: "data/valid/error/minimal_process.json"},
		{Name: "TestProcessErrorFull", Path: "data/valid/error/payload.json"},
		{Name: "TestProcessErrorNullValues", Path: "data/valid/error/null_values.json"},
		{Name: "TestProcessErrorAugmentedIP", Path: "data/valid/error/augmented_payload_backend.json"},
	}

	backendRequestInfoIgnoreTimestamp = []tests.RequestInfo{
		{Name: "TestProcessErrorMinimalPayloadException", Path: "data/valid/error/minimal_payload_exception.json"},
		{Name: "TestProcessErrorMinimalPayloadLog", Path: "data/valid/error/minimal_payload_log.json"},
	}

	frontendRequestInfo = []tests.RequestInfo{
		{Name: "TestProcessErrorFrontend", Path: "data/valid/error/frontend.json"},
		{Name: "TestProcessErrorFrontendNoSmap", Path: "data/valid/error/frontend_app.e2e-bundle.json"},
		{Name: "TestProcessErrorFrontendMinifiedSmap", Path: "data/valid/error/frontend_app.e2e-bundle.min.json"},
		{Name: "TestProcessErrorAugmentedUserAgentAndIP", Path: "data/valid/error/augmented_payload_frontend.json"},
	}
)

// ensure all valid documents pass through the whole validation and transformation process
func TestProcessorBackendOK(t *testing.T) {
	conf := config.Config{ExcludeFromGrouping: nil}
	tests.TestProcessRequests(t, er.NewProcessor(), conf, backendRequestInfo, map[string]string{})
}

func TestProcessorMinimalPayloadOK(t *testing.T) {
	conf := config.Config{ExcludeFromGrouping: nil}
	tests.TestProcessRequests(t, er.NewProcessor(), conf, backendRequestInfoIgnoreTimestamp, map[string]string{"@timestamp": "-"})
}

func TestProcessorFrontendOK(t *testing.T) {
	mapper := sourcemap.SmapMapper{Accessor: &fakeAcc{}}
	conf := config.Config{
		SmapMapper:          &mapper,
		LibraryPattern:      regexp.MustCompile("^test/e2e|~"),
		ExcludeFromGrouping: regexp.MustCompile("^\\s*$|^/webpack|^[/][^/]*$"),
	}
	tests.TestProcessRequests(t, er.NewProcessor(), conf, frontendRequestInfo, map[string]string{})
}

type fakeAcc struct {
	*testing.B
	benchmarkingConsumer *s.Consumer
}

var testSourcemapInfo = struct {
	name, file string
}{
	name: "http://localhost:8000/test/e2e/general-usecase/app.e2e-bundle.min.js",
	file: "app.e2e-bundle.min.js.map",
}

func (ac *fakeAcc) PreFetch() error {
	var err error
	ac.benchmarkingConsumer, err = ac.Fetch(sourcemap.Id{ServiceName: testSourcemapInfo.name})
	return err
}

func (ac *fakeAcc) Fetch(smapId sourcemap.Id) (*s.Consumer, error) {
	file := "bundle.js.map"
	if smapId.Path == testSourcemapInfo.name {
		// only not nil if PreFetch called
		// PreFetch only called from benchmarks, optionally
		if ac.B != nil && ac.benchmarkingConsumer != nil {
			return ac.benchmarkingConsumer, nil
		}
		file = testSourcemapInfo.file
	}
	current, _ := os.Getwd()
	path := filepath.Join(current, "../../../tests/data/valid/sourcemap/", file)
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return s.Parse("", fileBytes)
}
func (ac *fakeAcc) Remove(smapId sourcemap.Id) {}

func BenchmarkBackendProcessor(b *testing.B) {
	tests.BenchmarkProcessRequests(b, er.NewProcessor(), config.Config{ExcludeFromGrouping: nil}, backendRequestInfo)
	tests.BenchmarkProcessRequests(b, er.NewProcessor(), config.Config{ExcludeFromGrouping: nil}, backendRequestInfoIgnoreTimestamp)
}

func BenchmarkFrontendProcessor(b *testing.B) {
	accessor := &fakeAcc{B: b}
	if err := accessor.PreFetch(); err != nil {
		b.Fatal(err)
	}
	mapper := sourcemap.SmapMapper{Accessor: accessor}
	conf := config.Config{
		SmapMapper:          &mapper,
		LibraryPattern:      regexp.MustCompile("^test/e2e|~"),
		ExcludeFromGrouping: regexp.MustCompile("^\\s*$|^/webpack|^[/][^/]*$"),
	}
	tests.BenchmarkProcessRequests(b, er.NewProcessor(), conf, frontendRequestInfo)
}
