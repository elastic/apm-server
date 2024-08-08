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

//go:generate go run .

package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/tools/imports"

	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/generator"
)

const (
	basePath         = "github.com/elastic/apm-data"
	modeldecoderPath = "input/elasticapm/internal/modeldecoder"
)

var (
	importPath = path.Join(basePath, modeldecoderPath)
	inputDir   string
)

func main() {
	if _, file, _, ok := runtime.Caller(0); !ok {
		log.Fatal("failed to locate caller")
	} else {
		inputDir = filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	}
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
	generateCode(p, parsed, []string{"metadataRoot", "errorRoot", "metricsetRoot", "spanRoot", "transactionRoot", "logRoot"})
	generateJSONSchema(p, pkg, parsed, []string{"metadata", "errorEvent", "metricset", "span", "transaction"})
}

func generateV3RUM() {
	pkg := "rumv3"
	p := path.Join(importPath, pkg)
	parsed, err := generator.Parse(p)
	if err != nil {
		panic(err)
	}
	generateCode(p, parsed, []string{"metadataRoot", "errorRoot", "transactionRoot"})
	generateJSONSchema(p, pkg, parsed, []string{"metadata", "errorEvent", "span", "transaction"})
}

func generateCode(path string, parsed *generator.Parsed, root []string) {
	rootTypes := make([]string, len(root))
	for i := 0; i < len(root); i++ {
		rootTypes[i] = fmt.Sprintf("%s.%s", path, root[i])
	}
	code, err := generator.NewCodeGenerator(parsed, rootTypes)
	if err != nil {
		panic(err)
	}
	out := filepath.Join(parsed.Dir, "model_generated.go")
	b, err := code.Generate()
	if err != nil {
		panic(err)
	}
	formatted, err := imports.Process(out, b.Bytes(), nil)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(out, formatted, 0644); err != nil {
		panic(err)
	}
	log.Printf("generated %s", out)
}

func generateJSONSchema(pkgpath string, pkg string, parsed *generator.Parsed, root []string) {
	jsonSchema, err := generator.NewJSONSchemaGenerator(parsed)
	if err != nil {
		panic(err)
	}
	pkg = filepath.FromSlash(pkg)
	for _, rootEventName := range root {
		rootEvent := fmt.Sprintf("%s.%s", pkgpath, rootEventName)
		path := "docs/spec/" + pkg
		b, err := jsonSchema.Generate(path, rootEvent)
		if err != nil {
			panic(err)
		}
		outPath := filepath.Join(inputDir, filepath.FromSlash(path))
		out := filepath.Join(outPath, fmt.Sprintf("%s.json", strings.TrimSuffix(rootEventName, "Event")))
		if err := os.WriteFile(out, b.Bytes(), 0644); err != nil {
			panic(err)
		}
		log.Printf("generated %s", out)
	}
}
