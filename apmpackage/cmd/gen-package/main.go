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
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/elastic/apm-server/apmpackage"
)

var ecsDir string
var packageVersion string

func main() {
	flag.StringVar(&ecsDir, "ecsDir", "../ecs", "Path to the Elastic Common Schema repository")
	flag.StringVar(&packageVersion, "packageVersion", "0.1.0", "Package version")
	flag.Parse()
	clear(packageVersion)
	inputFields := apmpackage.GenerateFields(ecsDir, packageVersion)
	apmpackage.GenerateDocs(inputFields, packageVersion)
}

func clear(version string) {
	dir := filepath.Join("apmpackage/apm/", version, "/data_stream")
	fileInfo, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	for _, f := range fileInfo {
		if f.IsDir() {
			os.Remove(filepath.Join(dir, f.Name(), "fields/ecs.yml"))
			os.Remove(filepath.Join(dir, f.Name(), "fields/fields.yml"))
		}
	}
	ioutil.WriteFile(filepath.Join("apmpackage/apm/", version, "/docs/README.md"), nil, 0644)
}
