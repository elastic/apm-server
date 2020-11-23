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

package apmpackage

import (
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

func GenerateFields(ecsDir, version string) map[string]fields {

	// TODO get this from GH directly
	ecsFlatFields := loadECSFields(ecsDir)

	inputFieldsFiles := map[string]fields{
		"logs":    concatFields("model/error/_meta/fields.yml"),
		"metrics": concatFields("model/metricset/_meta/fields.yml", "model/profile/_meta/fields.yml"),
		"traces":  concatFields("model/transaction/_meta/fields.yml", "model/span/_meta/fields.yml"),
	}

	for streamType, inputFields := range inputFieldsFiles {
		var ecsFields fields
		var nonECSFields fields
		for _, fields := range populateECSInfo(ecsFlatFields, inputFields) {
			ecs, nonECS := splitECSFields(fields)
			if len(ecs.Fields) > 0 || ecs.IsECS {
				ecsFields = append(ecsFields, ecs)
			}
			if len(nonECS.Fields) > 0 || ecs.isNonECSLeaf() {
				nonECSFields = append(nonECSFields, nonECS)
			}
		}
		dataStreamFieldsPath := filepath.Join("apmpackage/apm/", version, "/data_stream", streamType, "/fields")
		var writeOutFields = func(fName string, data fields) {
			bytes, err := yaml.Marshal(&data)
			if err != nil {
				panic(err)
			}
			err = ioutil.WriteFile(filepath.Join(dataStreamFieldsPath, fName), bytes, 0644)
			if err != nil {
				panic(err)
			}
		}
		if len(ecsFields) > 0 {
			writeOutFields("ecs.yml", ecsFields)
		}
		if len(nonECSFields) > 0 {
			writeOutFields("fields.yml", nonECSFields)
		}
	}
	return inputFieldsFiles
}

func populateECSInfo(ecsFlatFields map[string]interface{}, inputFields fields) fields {
	var traverse func(string, fields) (fields, bool, bool)
	traverse = func(fName string, fs fields) (fields, bool, bool) {
		var ecsCount int
		for idx, field := range fs {
			fieldName := field.Name
			if fName != "" {
				fieldName = fName + "." + fieldName
			}
			if field.Type != "group" {
				_, ok := ecsFlatFields[fieldName]
				fs[idx].IsECS = ok
				if ok {
					ecsCount = ecsCount + 1
				}
			} else {
				fs[idx].Fields, fs[idx].HasECS, fs[idx].HasNonECS = traverse(fieldName, field.Fields)
			}
		}
		// first boolean returned indicates whether there is at least an ECS field in the group
		// second boolean returned indicates whether there is at least a non-ECS field in the group
		return fs, ecsCount > 0, ecsCount < len(fs)
	}
	ret, _, _ := traverse("", inputFields)
	return ret
}

func splitECSFields(parent field) (field, field) {
	ecsCopy := copyFieldRoot(parent)
	nonECSCopy := copyFieldRoot(parent)
	for _, field := range parent.Fields {
		ecsChild, nonECSChild := splitECSFields(field)
		if ecsChild.HasECS || ecsChild.IsECS {
			ecsCopy.Fields = append(ecsCopy.Fields, ecsChild)
		}
		if nonECSChild.HasNonECS || nonECSChild.isNonECSLeaf() {
			nonECSCopy.Fields = append(nonECSCopy.Fields, nonECSChild)
		}
	}
	return ecsCopy, nonECSCopy
}

// adapted from https://github.com/elastic/integrations/tree/master/dev/import-beats

func loadECSFields(ecsDir string) map[string]interface{} {
	path := filepath.Join(ecsDir, "generated/ecs/ecs_flat.yml")
	fields, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var ret map[string]interface{}
	err = yaml.Unmarshal(fields, &ret)
	if err != nil {
		panic(err)
	}
	return ret
}

func concatFields(fileNames ...string) fields {
	var ret fields
	for _, fname := range fileNames {
		fs := loadFieldsFile(fname)
		for _, key := range fs {
			ret = append(ret, key.Fields...)
		}
	}
	return ret
}

func loadFieldsFile(path string) fields {
	fields, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	var fs []field
	err = yaml.Unmarshal(fields, &fs)
	if err != nil {
		panic(err)
	}
	return overrideFieldValues(fs)
}

func overrideFieldValues(fs fields) fields {
	var ret []field
	for _, f := range fs {
		if f.Type == "" {
			f.Type = "keyword"
		}
		f.Fields = overrideFieldValues(f.Fields)
		ret = append(ret, f)
	}
	return ret
}
