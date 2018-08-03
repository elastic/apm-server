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

	perr "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/transform"
)

var (
	backendRequestInfo = []tests.RequestInfo{
		{Name: "TestProcessErrorMinimalService", Path: "../testdata/error/minimal_service.json"},
		{Name: "TestProcessErrorMinimalProcess", Path: "../testdata/error/minimal_process.json"},
		{Name: "TestProcessErrorFull", Path: "../testdata/error/payload.json"},
		{Name: "TestProcessErrorNullValues", Path: "../testdata/error/null_values.json"},
		{Name: "TestProcessErrorAugmentedIP", Path: "../testdata/error/augmented_payload_backend.json"},
	}

	backendRequestInfoIgnoreTimestamp = []tests.RequestInfo{
		{Name: "TestProcessErrorMinimalPayloadException", Path: "../testdata/error/minimal_payload_exception.json"},
		{Name: "TestProcessErrorMinimalPayloadLog", Path: "../testdata/error/minimal_payload_log.json"},
	}

	rumRequestInfo = []tests.RequestInfo{
		{Name: "TestProcessErrorRum", Path: "../testdata/error/rum.json"},
		{Name: "TestProcessErrorRumNoSmap", Path: "../testdata/error/rum_app.e2e-bundle.json"},
		{Name: "TestProcessErrorRumMinifiedSmap", Path: "../testdata/error/rum_app.e2e-bundle.min.json"},
		{Name: "TestProcessErrorAugmentedUserAgentAndIP", Path: "../testdata/error/augmented_payload_rum.json"},
	}
)

// ensure all valid documents pass through the whole validation and transformation process
func TestProcessorBackendOK(t *testing.T) {
	tctx := transform.Context{}
	tests.TestProcessRequests(t, perr.Processor, tctx, backendRequestInfo, map[string]string{})
}

func TestProcessorMinimalPayloadOK(t *testing.T) {
	tctx := transform.Context{}
	tests.TestProcessRequests(t, perr.Processor, tctx, backendRequestInfoIgnoreTimestamp, map[string]string{"@timestamp": "-"})
}

func TestProcessorRumOK(t *testing.T) {
	mapper := sourcemap.SmapMapper{Accessor: &fakeAcc{}}
	conf := transform.Config{
		SmapMapper:          &mapper,
		LibraryPattern:      regexp.MustCompile("^test/e2e|~"),
		ExcludeFromGrouping: regexp.MustCompile("^\\s*$|^/webpack|^[/][^/]*$"),
	}
	tctx := transform.Context{Config: conf}
	tests.TestProcessRequests(t, perr.Processor, tctx, rumRequestInfo, map[string]string{})
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
	path := filepath.Join(current, "../../../testdata/sourcemap/", file)
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return s.Parse("", fileBytes)
}
func (ac *fakeAcc) Remove(smapId sourcemap.Id) {}

func BenchmarkBackendProcessor(b *testing.B) {
	tctx := transform.Context{Config: transform.Config{ExcludeFromGrouping: nil}}
	tests.BenchmarkProcessRequests(b, perr.Processor, tctx, backendRequestInfo)
	tests.BenchmarkProcessRequests(b, perr.Processor, tctx, backendRequestInfoIgnoreTimestamp)
}

func BenchmarkRumProcessor(b *testing.B) {
	accessor := &fakeAcc{B: b}
	if err := accessor.PreFetch(); err != nil {
		b.Fatal(err)
	}
	mapper := sourcemap.SmapMapper{Accessor: accessor}
	conf := transform.Config{
		SmapMapper:          &mapper,
		LibraryPattern:      regexp.MustCompile("^test/e2e|~"),
		ExcludeFromGrouping: regexp.MustCompile("^\\s*$|^/webpack|^[/][^/]*$"),
	}
	tctx := transform.Context{Config: conf}
	tests.BenchmarkProcessRequests(b, perr.Processor, tctx, rumRequestInfo)
}
