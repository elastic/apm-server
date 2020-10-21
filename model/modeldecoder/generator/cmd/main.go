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
	"os"
	"path"
	"path/filepath"

	"golang.org/x/tools/imports"

	"github.com/elastic/apm-server/model/modeldecoder/generator"
)

const (
	basePath         = "github.com/elastic/apm-server"
	modeldecoderPath = "model/modeldecoder"

	pkgV2    = "v2"
	pkgV3RUM = "rumv3"
)

var (
	importPath = path.Join(basePath, modeldecoderPath)
	typPath    = path.Join(importPath, "nullable")
)

func main() {
	genV2()
	genRUMV3()
}

func genV2() {
	rootObjs := []string{"metadataRoot", "errorRoot", "metricsetRoot", "spanRoot", "transactionRoot"}
	out := filepath.Join(filepath.FromSlash(modeldecoderPath), pkgV2, "model_generated.go")
	gen, err := generator.NewGenerator(importPath, pkgV2, typPath, rootObjs)
	if err != nil {
		panic(err)
	}
	generate(gen, out)
}

func genRUMV3() {
	rootObjs := []string{"metadataRoot", "errorRoot", "transactionRoot"}
	out := filepath.Join(filepath.FromSlash(modeldecoderPath), pkgV3RUM, "model_generated.go")
	gen, err := generator.NewGenerator(importPath, pkgV3RUM, typPath, rootObjs)
	if err != nil {
		panic(err)
	}
	generate(gen, out)
}

type gen interface {
	Generate() (bytes.Buffer, error)
}

func generate(g gen, p string) {
	b, err := g.Generate()
	if err != nil {
		panic(err)
	}
	processed, err := imports.Process(p, b.Bytes(), nil)
	if err != nil {
		panic(err)
	}
	f, err := os.Create(p)
	if err != nil {
		panic(err)
	}
	if _, err := f.Write(processed); err != nil {
		panic(err)
	}
}
