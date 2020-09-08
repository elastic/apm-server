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
	"go/format"
	"os"
	"path"
	"path/filepath"

	"github.com/elastic/apm-server/model/modeldecoder/generator"
)

const (
	basePath         = "github.com/elastic/apm-server"
	modeldecoderPath = "model/modeldecoder"
)

var (
	importPath = path.Join(basePath, modeldecoderPath)
	typPath    = path.Join(importPath, "nullable")
)

func main() {
	genV2Models()
	genRUMV3Models()
}

func genV2Models() {
	pkg := "v2"
	rootObjs := []string{"metadataRoot"}
	out := filepath.Join(filepath.FromSlash(modeldecoderPath), pkg, "model_generated.go")
	gen, err := generator.NewGenerator(importPath, pkg, typPath, rootObjs)
	if err != nil {
		panic(err)
	}
	generate(gen, out)
}

func genRUMV3Models() {
	pkg := "rumv3"
	rootObjs := []string{"metadataRoot"}
	out := filepath.Join(filepath.FromSlash(modeldecoderPath), pkg, "model_generated.go")
	gen, err := generator.NewGenerator(importPath, pkg, typPath, rootObjs)
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
	fmtd, err := format.Source(b.Bytes())
	if err != nil {
		panic(err)
	}
	f, err := os.Create(p)
	if err != nil {
		panic(err)
	}
	if _, err := f.Write(fmtd); err != nil {
		panic(err)
	}
}
