// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

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
		{Name: "TestProcessErrorMinimalService", Path: "data/error/minimal_service.json"},
		{Name: "TestProcessErrorMinimalProcess", Path: "data/error/minimal_process.json"},
		{Name: "TestProcessErrorFull", Path: "data/error/payload.json"},
		{Name: "TestProcessErrorNullValues", Path: "data/error/null_values.json"},
		{Name: "TestProcessErrorAugmentedIP", Path: "data/error/augmented_payload_backend.json"},
	}

	backendRequestInfoIgnoreTimestamp = []tests.RequestInfo{
		{Name: "TestProcessErrorMinimalPayloadException", Path: "data/error/minimal_payload_exception.json"},
		{Name: "TestProcessErrorMinimalPayloadLog", Path: "data/error/minimal_payload_log.json"},
	}

	frontendRequestInfo = []tests.RequestInfo{
		{Name: "TestProcessErrorFrontend", Path: "data/error/frontend.json"},
		{Name: "TestProcessErrorFrontendNoSmap", Path: "data/error/frontend_app.e2e-bundle.json"},
		{Name: "TestProcessErrorFrontendMinifiedSmap", Path: "data/error/frontend_app.e2e-bundle.min.json"},
		{Name: "TestProcessErrorAugmentedUserAgentAndIP", Path: "data/error/augmented_payload_frontend.json"},
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
	path := filepath.Join(current, "../../../tests/data/sourcemap/", file)
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
