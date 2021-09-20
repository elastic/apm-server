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
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v2"

	"github.com/elastic/ecs/code/go/ecs"
)

func generateFields() map[string]fieldMap {

	ecsFlatFields := loadECSFields()

	inputFieldsFiles := map[string]fieldMap{
		"error_logs":       readFields("model/error/_meta/fields.yml"),
		"internal_metrics": readFields("model/metricset/_meta/fields.yml", "x-pack/apm-server/fields/_meta/fields.yml"),
		"profile_metrics":  readFields("model/profile/_meta/fields.yml"),
		"traces":           readFields("model/transaction/_meta/fields.yml", "model/span/_meta/fields.yml"),
	}

	appMetrics := readFields("model/metricset/_meta/fields.yml", "x-pack/apm-server/fields/_meta/fields.yml")
	delete(appMetrics, "transaction")
	delete(appMetrics, "span")
	delete(appMetrics, "event")
	inputFieldsFiles["app_metrics"] = appMetrics

	for streamType, fields := range inputFieldsFiles {
		log.Printf("%s", streamType)
		populateECSInfo(ecsFlatFields, fields)
		for _, field := range fields {
			log.Printf(" - %s (%s)", field.Name, field.Type)
		}
		ecsFields, nonECSFields := splitFields(fields)

		var topLevelECSFields, topLevelNonECSFields []field
		for _, field := range ecsFields {
			topLevelECSFields = append(topLevelECSFields, field.field)
		}
		for _, field := range nonECSFields {
			topLevelNonECSFields = append(topLevelNonECSFields, field.field)
		}
		sortFields(topLevelECSFields)
		sortFields(topLevelNonECSFields)
		writeFields(streamType, "ecs.yml", topLevelECSFields)
		writeFields(streamType, "fields.yml", topLevelNonECSFields)
	}
	return inputFieldsFiles
}

func writeFields(streamType, filename string, data []field) {
	if len(data) == 0 {
		return
	}
	bytes, err := yaml.Marshal(data)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(filepath.Join(fieldsPath(streamType), filename), bytes, 0644)
	if err != nil {
		panic(err)
	}
}

// populateECSInfo sets the IsECS property of each field. Group fields
// will have IsECS set only if all sub-fields have IsECS set.
func populateECSInfo(ecsFlatFields map[string]interface{}, inputFields fieldMap) {
	var traverse func(path string, fields fieldMap) (ecsOnly bool)
	traverse = func(path string, fields fieldMap) bool {
		ecsOnly := true
		for name, field := range fields {
			fieldName := field.Name
			if path != "" {
				fieldName = path + "." + fieldName
			}
			if field.Type != "group" {
				_, field.IsECS = ecsFlatFields[fieldName]
			} else {
				field.IsECS = traverse(fieldName, field.fields)
			}
			fields[name] = field
			ecsOnly = ecsOnly && field.IsECS
		}
		return ecsOnly
	}
	traverse("", inputFields)
}

// splitFields splits fields into ECS and non-ECS fieldMaps.
func splitFields(fields fieldMap) (ecsFields, nonECSFields fieldMap) {
	ecsFields = make(fieldMap)
	nonECSFields = make(fieldMap)
	for name, field := range fields {
		if field.IsECS {
			ecsFields[name] = field
			continue
		} else if field.Type != "group" {
			nonECSFields[name] = field
			continue
		}
		subECSFields, subNonECSFields := splitFields(field.fields)
		fieldCopy := field.field
		fieldCopy.Fields = nil // recreated by update calls
		if len(subECSFields) > 0 {
			ecsFields[name] = fieldMapItem{field: fieldCopy, fields: subECSFields}
			ecsFields.update(fieldCopy)
		}
		nonECSFields[name] = fieldMapItem{field: fieldCopy, fields: subNonECSFields}
		nonECSFields.update(fieldCopy)
	}
	return ecsFields, nonECSFields
}

func loadECSFields() map[string]interface{} {
	url := "https://raw.githubusercontent.com/elastic/ecs/v" + ecs.Version + "/generated/ecs/ecs_flat.yml"
	// TODO cache this to avoid fetching each time
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var ret map[string]interface{}
	err = yaml.NewDecoder(resp.Body).Decode(&ret)
	if err != nil {
		panic(err)
	}
	return ret
}

// readFields reads fields from all of the specified files,
// merging them into a fieldMap.
func readFields(fileNames ...string) fieldMap {
	fields := make(fieldMap)
	for _, fname := range fileNames {
		for _, key := range loadFieldsFile(fname) {
			for _, field := range key.Fields {
				fields.update(field)
			}
		}
	}
	return fields
}

func flattenFields(fields fieldMap) []field {
	var flattened []field
	var traverse func(path string, fields fieldMap)
	traverse = func(path string, fields fieldMap) {
		for name, field := range fields {
			full := path
			if full != "" {
				full += "."
			}
			full += name
			field.Name = full
			if field.Type == "group" {
				traverse(full, field.fields)
			} else {
				flattened = append(flattened, field.field)
			}
		}
	}
	traverse("", fields)
	sortFields(flattened)
	return flattened
}

type fieldMap map[string]fieldMapItem

type fieldMapItem struct {
	field
	fields fieldMap
}

func (m fieldMap) update(f field) {
	if f.DynamicTemplate {
		// We don't add dynamic_template "fields" to the
		// integration package; they are manually defined
		// in the data stream manifest.
		return
	}

	item := m[f.Name]
	item.field = f
	if item.fields == nil {
		item.fields = make(fieldMap)
	}
	for _, f := range f.Fields {
		item.fields.update(f)
	}
	// Update the Fields slice, in case of merges.
	item.Fields = item.Fields[:0]
	for _, f := range item.fields {
		item.Fields = append(item.Fields, f.field)
	}
	sortFields(item.Fields)
	m[f.Name] = item
}

func loadFieldsFile(path string) []field {
	fields, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var fs []field
	if err := yaml.Unmarshal(fields, &fs); err != nil {
		panic(err)
	}
	return fs
}

func sortFields(fields []field) {
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})
}
