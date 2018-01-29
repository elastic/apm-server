package package_tests

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	s "github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/processor"
	er "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/tests"
)

// ensure all valid documents pass through the whole validation and transformation process
func TestProcessorBackendOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessErrorMinimalPayloadException", Path: "data/valid/error/minimal_payload_exception.json"},
		{Name: "TestProcessErrorMinimalPayloadLog", Path: "data/valid/error/minimal_payload_log.json"},
		{Name: "TestProcessErrorMinimalService", Path: "data/valid/error/minimal_service.json"},
		{Name: "TestProcessErrorMinimalProcess", Path: "data/valid/error/minimal_process.json"},
		{Name: "TestProcessErrorFull", Path: "data/valid/error/payload.json"},
		{Name: "TestProcessErrorNullValues", Path: "data/valid/error/null_values.json"},
	}
	conf := processor.Config{ExcludeFromGrouping: nil}
	tests.TestProcessRequests(t, er.NewProcessor(&conf), requestInfo, map[string]string{})
}

func TestProcessorFrontendMinifiedSmapOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessErrorFrontend", Path: "data/valid/error/frontend.json"},
		{Name: "TestProcessErrorFrontendNoSmap", Path: "data/valid/error/frontend_app.e2e-bundle.json"},
		{Name: "TestProcessErrorFrontendMinifiedSmap", Path: "data/valid/error/frontend_app.e2e-bundle.min.json"},
	}
	mapper := sourcemap.SmapMapper{Accessor: &fakeAcc{}}
	conf := processor.Config{
		SmapMapper:          &mapper,
		LibraryPattern:      regexp.MustCompile("^test/e2e|~"),
		ExcludeFromGrouping: regexp.MustCompile("^\\s*$|^/webpack|^[/][^/]*$"),
	}
	tests.TestProcessRequests(t, er.NewProcessor(&conf), requestInfo, map[string]string{})
}

// ensure invalid documents fail the json schema validation already
func TestProcessorFailedValidation(t *testing.T) {
	data, err := tests.LoadInvalidData("error")
	assert.Nil(t, err)
	err = er.NewProcessor(nil).Validate(data)
	assert.NotNil(t, err)
}

type fakeAcc struct{}

func (ac *fakeAcc) Fetch(smapId sourcemap.Id) (*s.Consumer, error) {
	file := "bundle.js.map"
	if smapId.Path == "http://localhost:8000/test/e2e/general-usecase/app.e2e-bundle.min.js" {
		file = "app.e2e-bundle.min.js.map"
	}

	current, _ := os.Getwd()
	path := filepath.Join(current, "../../../tests/data/valid/sourcemap/", file)
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return s.Parse("", fileBytes)
}
func (a *fakeAcc) Remove(smapId sourcemap.Id) {}
