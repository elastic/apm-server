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
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/tools/imports"

	"github.com/elastic/apm-server/model/modeldecoder/generator"
)

const (
	basePath         = "github.com/elastic/apm-server"
	modeldecoderPath = "model/modeldecoder"
	jsonSchemaPath   = "docs/spec/"
)

var (
	importPath = path.Join(basePath, modeldecoderPath)
)

func main() {
	generateV2()
	generateV3RUM()
}

func generateV2() {
	pkg := "v2"
	p := path.Join(importPath, pkg)
	parsed, err := generator.Parse(p)
	if err != nil {
		panic(err)
	}
	generateCode(p, pkg, parsed, []string{"metadataRoot", "errorRoot", "metricsetRoot", "spanRoot", "transactionRoot"})
}

func generateV3RUM() {
	pkg := "rumv3"
	p := path.Join(importPath, pkg)
	parsed, err := generator.Parse(p)
	if err != nil {
		panic(err)
	}
	generateCode(p, pkg, parsed, []string{"metadataRoot", "errorRoot", "metricsetRoot", "transactionRoot"})
}

func generateCode(path string, pkg string, parsed *generator.Parsed, root []string) {
	rootTypes := make([]string, len(root))
	for i := 0; i < len(root); i++ {
		rootTypes[i] = fmt.Sprintf("%s.%s", path, root[i])
	}
	code, err := generator.NewCodeGenerator(parsed, rootTypes)
	if err != nil {
		panic(err)
	}
	out := filepath.Join(filepath.FromSlash(modeldecoderPath), pkg, "model_generated.go")
	b, err := code.Generate()
	if err != nil {
		panic(err)
	}
	print(b, out)
}

func print(b bytes.Buffer, p string) {
	f, err := os.Create(p)
	if err != nil {
		panic(err)
	}
	var out = b.Bytes()
	if out, err = imports.Process(p, out, nil); err != nil {
		panic(err)
	}
	if _, err := f.Write(out); err != nil {
		panic(err)
	}
}
