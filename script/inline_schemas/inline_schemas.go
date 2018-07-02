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
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const basePath = "./docs/spec/"

func main() {
	schemaPaths := []struct {
		path, schemaOut, goVariable, packageName string
	}{
		{"errors/payload.json", "processor/error/schema.go", "errorSchema", "error"},
		{"transactions/payload.json", "processor/transaction/schema.go", "transactionSchema", "transaction"},
		{"sourcemaps/payload.json", "processor/sourcemap/schema.go", "sourcemapSchema", "sourcemap"},
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

		publicSchema := fmt.Sprintf("func Schema() string {\n\treturn %s\n}\n", schemaInfo.goVariable)
		goScript := fmt.Sprintf("package %s\n\n%s\nvar %s = `%s`\n", schemaInfo.packageName, publicSchema, schemaInfo.goVariable, schema)
		err = ioutil.WriteFile(schemaInfo.schemaOut, []byte(goScript), 0644)
		if err != nil {
			panic(err)
		}
	}
	if checkHeader := os.Getenv("CHECK_HEADERS_DISABLED"); checkHeader == "" {
		cmd := exec.Command("go-licenser")
		cmd.Run()
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
