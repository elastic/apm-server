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

package v2

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"

	"github.com/elastic/apm-server/decoder"
)

type testFile struct {
	name  string
	r     io.Reader
	valid bool
}

func TestJSONSchema(t *testing.T) {
	rootDir := filepath.Join("..", "..", "..")
	// read and organize test data
	testdataDir := filepath.Join(rootDir, "testdata", "intake-v2")
	var files []testFile
	err := filepath.Walk(testdataDir, func(p string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}
		// ignore files with invalid json
		if info.Name() == "invalid-json-metadata.ndjson" {
			return nil
		}
		var valid bool
		if !strings.HasPrefix(info.Name(), "invalid") {
			valid = true
		}
		r, err := os.Open(p)
		if err != nil {
			return err
		}
		files = append(files, testFile{name: info.Name(), r: r, valid: valid})
		return nil
	})
	require.NoError(t, err)
	// read and organize schemas
	schemaDir := filepath.Join(rootDir, "docs", "spec", "v2")
	schemas := map[string]string{}
	err = filepath.Walk(schemaDir, func(p string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}
		f, err := ioutil.ReadFile(p)
		if err != nil {
			return err
		}
		schemas[strings.TrimSuffix(info.Name(), ".json")] = string(f)
		return nil
	})
	require.NoError(t, err)

	for _, f := range files {
		// validate data against schemas
		t.Run(f.name, func(t *testing.T) {
			var data map[string]json.RawMessage
			dec := decoder.NewNDJSONStreamDecoder(f.r, 300*1024)
			b, err := dec.ReadAhead()
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(b, &data))
			for k := range data {
				schema, ok := schemas[k]
				if !ok && !f.valid {
					// if no schema exists for invalid test event just ignore it
					continue
				}
				t.Run(k, func(t *testing.T) {
					schemaLoader := gojsonschema.NewStringLoader(schema)
					dataLoader := gojsonschema.NewStringLoader(string(data[k]))
					result, err := gojsonschema.Validate(schemaLoader, dataLoader)
					require.NoError(t, err)
					expected := f.valid
					if k == "metadata" && f.name != "invalid-metadata.ndjson" {
						// all invalid test files contain valid metadata
						expected = true
					}
					assert.Equal(t, expected, result.Valid(), result.Errors())
				})
			}
		})
	}
}
