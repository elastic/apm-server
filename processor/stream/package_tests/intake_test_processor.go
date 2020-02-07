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
	"bufio"
	"io"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/tests/loader"
)

const lrSize = 100 * 1024

func getReader(path string) (*decoder.NDJSONStreamReader, error) {
	reader, err := loader.LoadDataAsStream(path)
	if err != nil {
		return nil, err
	}
	lr := decoder.NewLineReader(bufio.NewReaderSize(reader, lrSize), lrSize)
	return decoder.NewNDJSONStreamReader(lr), nil
}

func readEvents(reader *decoder.NDJSONStreamReader) ([]map[string]interface{}, error) {
	var (
		err    error
		e      map[string]interface{}
		events []map[string]interface{}
	)

	for err != io.EOF {
		e, err = reader.Read()
		if err != nil && err != io.EOF {
			return events, err
		}
		if e != nil {
			events = append(events, e)
		}
	}
	return events, nil
}

func loadPayload(path string) ([]map[string]interface{}, error) {
	ndjson, err := getReader(path)
	if err != nil {
		return nil, err
	}
	// read and discard metadata
	ndjson.Read()
	return readEvents(ndjson)
}

func loadEvents(path string) (interface{}, error) {
	return loadPayload(path)
}

func loadEvent(path string, index int) map[string]interface{} {
	events, _ := loadPayload(path)
	return events[index]
}
