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

package loader

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/elastic/apm-server/decoder"
)

func findFile(fileName string) (string, error) {
	_, current, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(current), "..", fileName), nil
}

func fileReader(filePath string, err error) (io.ReadCloser, error) {
	if err != nil {
		return nil, err
	}
	return os.Open(filePath)
}

func readFile(filePath string, err error) ([]byte, error) {
	var f io.Reader
	f, err = fileReader(filePath, err)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(f)
}

func LoadData(file string) (map[string]interface{}, error) {
	return unmarshalData(findFile(file))
}

func LoadDataAsBytes(fileName string) ([]byte, error) {
	return readFile(findFile(fileName))
}

func LoadValidDataAsBytes(processorName string) ([]byte, error) {
	return readFile(buildPath(processorName, true))
}

func LoadValidData(processorName string) (map[string]interface{}, error) {
	return unmarshalData(buildPath(processorName, true))
}

func LoadInvalidData(processorName string) (map[string]interface{}, error) {
	return unmarshalData(buildPath(processorName, false))
}

func buildPath(processorName string, validData bool) (string, error) {
	valid := "valid"
	if !validData {
		valid = "invalid"
	}

	var file string
	switch processorName {
	case "error":
		if validData {
			file = "error/payload.json"
		} else {
			file = "error_payload/no_service.json"
		}
	case "transaction":
		if validData {
			file = "transaction/payload.json"
		} else {
			file = "transaction_payload/no_service.json"
		}
	case "sourcemap":
		if validData {
			file = "sourcemap/payload.json"
		} else {
			file = "sourcemap/no_bundle_filepath.json"
		}
	default:
		return "", errors.New("data type not specified")
	}
	return findFile(filepath.Join("data", valid, file))
}

func unmarshalData(filePath string, err error) (map[string]interface{}, error) {
	var data map[string]interface{}
	var r io.ReadCloser
	r, err = fileReader(filePath, err)
	if err != nil {
		return data, err
	}
	return decoder.DecodeJSONData(r)
}
