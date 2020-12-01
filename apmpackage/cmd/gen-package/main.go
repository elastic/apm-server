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
	"errors"
	"fmt"
	"github.com/elastic/apm-server/cmd"
	"github.com/elastic/beats/v7/libbeat/common"
	"io/ioutil"
	"log"
	"os"
)

var versionMapping = map[string]string{
	"7.11": "0.1.0",
	"8.0":  "0.1.0",
}

func main() {
	stackVersion := common.MustNewVersion(cmd.DefaultSettings().Version)
	shortVersion := fmt.Sprintf("%d.%d", stackVersion.Major, stackVersion.Minor)
	packageVersion, ok := versionMapping[shortVersion]
	if !ok {
		panic(errors.New("package can't be generated for current apm-server version"))
	}
	clear(packageVersion)
	inputFields := generateFields(packageVersion)
	generateDocs(inputFields, packageVersion)
	log.Printf("Package fields and docs generated for version %s (stack %s)", packageVersion, stackVersion.String())
}

func clear(version string) {
	fileInfo, err := ioutil.ReadDir(dataStreamPath(version))
	if err != nil {
		log.Printf("NOTE: if you are adding a new package version, you must create the folder"+
			" `apmpackage/apm/%s/` and copy all the contents from the previous version.", version)
		panic(err)
	}
	for _, f := range fileInfo {
		if f.IsDir() {
			os.Remove(ecsFilePath(version, f.Name()))
			os.Remove(fieldsFilePath(version, f.Name()))
		}
	}
	ioutil.WriteFile(docsFilePath(version), nil, 0644)
}
