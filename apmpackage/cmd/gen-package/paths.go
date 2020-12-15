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

import "path/filepath"

var docsTemplateFilePath = "apmpackage/docs/README.template.md"

func docsFilePath(version string) string {
	return filepath.Join("apmpackage/apm/", version, "/docs/README.md")
}

func pipelinesPath(version, dataStream string) string {
	return filepath.Join("apmpackage/apm/", version, "/data_stream/", dataStream, "/elasticsearch/ingest_pipeline/")
}

func dataStreamPath(version string) string {
	return filepath.Join("apmpackage/apm/", version, "/data_stream/")
}

func fieldsPath(version, dataStream string) string {
	return filepath.Join(dataStreamPath(version), dataStream, "fields/")
}

func ecsFilePath(version, dataStream string) string {
	return filepath.Join(fieldsPath(version, dataStream), "ecs.yml")
}

func fieldsFilePath(version, dataStream string) string {
	return filepath.Join(fieldsPath(version, dataStream), "fields.yml")
}

func baseFieldsFilePath(version, dataStream string) string {
	return filepath.Join(fieldsPath(version, dataStream), "base-fields.yml")
}
