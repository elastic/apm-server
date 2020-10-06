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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/elastic/apm-server/decoder"
)

func LoadData(file string) (map[string]interface{}, error) {
	return unmarshalData(FindFile(file))
}

func LoadDataAsBytes(fileName string) ([]byte, error) {
	return readFile(FindFile(fileName))
}

func LoadDataAsStream(file string) (io.ReadCloser, error) {
	return fileReader(FindFile(file))
}

func FindFile(fileInfo ...string) (string, error) {
	_, current, _, _ := runtime.Caller(0)
	f := []string{filepath.Dir(current), ".."}
	f = append(f, fileInfo...)
	p := filepath.Join(f...)
	_, err := os.Stat(p)
	return p, err
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

func unmarshalData(filePath string, err error) (map[string]interface{}, error) {
	var r io.ReadCloser
	r, err = fileReader(filePath, err)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var m map[string]interface{}
	err = decoder.NewJSONDecoder(r).Decode(&m)
	return m, err
}
