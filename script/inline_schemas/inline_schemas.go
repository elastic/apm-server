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

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const basePath = "./docs/spec/"

func main() {
	schemaPaths := []struct {
		path, schemaOut, varName string
	}{
		{"metadata.json", "model/metadata/generated/schema/metadata.go", "ModelSchema"},
		{"rum_v3_metadata.json", "model/metadata/generated/schema/rum_v3_metadata.go", "RUMV3Schema"},
		{"errors/error.json", "model/error/generated/schema/error.go", "ModelSchema"},
		{"transactions/transaction.json", "model/transaction/generated/schema/transaction.go", "ModelSchema"},
		{"spans/span.json", "model/span/generated/schema/span.go", "ModelSchema"},
		{"metricsets/metricset.json", "model/metricset/generated/schema/metricset.go", "ModelSchema"},
		{"errors/rum_v3_error.json", "model/error/generated/schema/rum_v3_error.go", "RUMV3Schema"},
		{"transactions/rum_v3_transaction.json", "model/transaction/generated/schema/rum_v3_transaction.go", "RUMV3Schema"},
		{"spans/rum_v3_span.json", "model/span/generated/schema/rum_v3_span.go", "RUMV3Schema"},
		{"metricsets/rum_v3_metricset.json", "model/metricset/generated/schema/rum_v3_metricset.go", "RUMV3Schema"},
	}
	for _, schemaInfo := range schemaPaths {
		file := filepath.Join(filepath.Dir(basePath), schemaInfo.path)
		schemaBytes, err := ioutil.ReadFile(file)
		if err != nil {
			panic(err)
		}

		schema, err := replaceRef(filepath.Dir(file), string(schemaBytes))
		if err != nil {
			panic(err)
		}

		goScript := fmt.Sprintf("package schema\n\nconst %s = `%s`\n", schemaInfo.varName, schema)
		err = os.MkdirAll(path.Dir(schemaInfo.schemaOut), os.ModePerm)
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(schemaInfo.schemaOut, []byte(goScript), 0644)
		if err != nil {
			panic(err)
		}
	}
}

var re = regexp.MustCompile(`\"\$ref\": \"(.*?.json)\"`)
var findAndReplace = map[string]string{}

func replaceRef(currentDir string, schema string) (string, error) {
	matches := re.FindAllStringSubmatch(schema, -1)
	for _, match := range matches {
		pattern := escapePattern(match[0])
		if _, ok := findAndReplace[pattern]; !ok {
			s, err := read(currentDir, match[1])
			if err != nil {
				panic(err)
			}
			findAndReplace[pattern] = trimSchemaPart(s)
		}

		re := regexp.MustCompile(pattern)
		schema = re.ReplaceAllLiteralString(schema, findAndReplace[pattern])
	}
	return schema, nil
}

func read(currentRelativePath string, filePath string) (string, error) {
	path := filepath.Join(currentRelativePath, filePath)
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return replaceRef(filepath.Dir(path), string(file))
}

var reDollar = regexp.MustCompile(`\$`)
var reQuote = regexp.MustCompile(`\"`)

func escapePattern(pattern string) string {
	pattern = reDollar.ReplaceAllLiteralString(pattern, `\$`)
	return reQuote.ReplaceAllLiteralString(pattern, `\"`)
}

func trimSchemaPart(part string) string {
	part = strings.Trim(part, "\n")
	part = strings.Trim(part, "\b")
	part = strings.TrimSuffix(part, "}")
	part = strings.TrimPrefix(part, "{")
	part = strings.Trim(part, "\n")
	return strings.Trim(part, "\b")
}
