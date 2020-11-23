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

	"github.com/elastic/apm-server/apmpackage"
)

var packageVersion string

func main() {
	flag.StringVar(&packageVersion, "packageVersion", "0.1.0", "Package version")
	flag.Parse()
	clear(packageVersion)
	inputFields := apmpackage.GenerateFields(packageVersion)
	apmpackage.GenerateDocs(inputFields, packageVersion)
}

func clear(version string) {
	fileInfo, err := ioutil.ReadDir(apmpackage.DataStreamPath(version))
	if err != nil {
		panic(err)
	}
	for _, f := range fileInfo {
		if f.IsDir() {
			os.Remove(apmpackage.ECSFilePath(version, f.Name()))
			os.Remove(apmpackage.FieldsFilePath(version, f.Name()))
		}
	}
	ioutil.WriteFile(apmpackage.DocsFilePath(version), nil, 0644)
}
