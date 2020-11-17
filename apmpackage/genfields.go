package apmpackage

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

func GenerateFields(ecsDir string) {

	// TODO get this from GH directly
	ecsFlatFields := loadECSFields(ecsDir)

	inputFieldsFiles := map[string][]string{
		"logs":    {"model/error/_meta/fields.yml"},
		"metrics": {"model/metricset/_meta/fields.yml", "model/profile/_meta/fields.yml"},
		"traces":  {"_meta/fields.common.yml", "model/transaction/_meta/fields.yml", "model/span/_meta/fields.yml"},
	}

	for streamType, fieldsFiles := range inputFieldsFiles {
		var ecsFields []FieldDefinition
		var nonECSFields []FieldDefinition
		for _, fieldsFile := range fieldsFiles {
			for _, fields := range populateECSInfo(ecsFlatFields, loadFields(fieldsFile)) {
				ecs, nonECS := splitECSFields(fields)
				if len(ecs.Fields) > 0 || ecs.IsECS {
					ecsFields = append(ecsFields, ecs)
				}
				// TODO probably should check type
				if len(nonECS.Fields) > 0 || !ecs.IsECS {
					nonECSFields = append(nonECSFields, nonECS)
				}
			}
		}
		// TODO handle version better
		dataStreamFieldsPath := filepath.Join("apmpackage/apm/0.1.0/data_stream", streamType, "fields")
		var writeOutFields = func(fName string, data []FieldDefinition) {
			data = rmGroupDescriptions(data)
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
}

// TODO move functionality to loadDefaultFieldValues
func rmGroupDescriptions(data []FieldDefinition) []FieldDefinition {
	var ret []FieldDefinition
	for _, f := range data {
		copy := f
		if f.Type == "group" {
			copy.Description = ""
		}
		copy.Fields = rmGroupDescriptions(copy.Fields)
		ret = append(ret, copy)
	}
	return ret
}

func populateECSInfo(ecsFlatFields map[string]interface{}, fields []FieldDefinition) []FieldDefinition {
	var traverse func(string, []FieldDefinition) ([]FieldDefinition, bool, bool)
	traverse = func(fName string, fs []FieldDefinition) ([]FieldDefinition, bool, bool) {
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
	ret, _, _ := traverse("", fields)
	return ret
}

func splitECSFields(parent FieldDefinition) (FieldDefinition, FieldDefinition) {
	ecsCopy := copyFieldRoot(parent)
	nonECSCopy := copyFieldRoot(parent)
	for _, field := range parent.Fields {
		ecsChild, nonECSChild := splitECSFields(field)
		if ecsChild.HasECS || ecsChild.IsECS {
			ecsCopy.Fields = append(ecsCopy.Fields, ecsChild)
		}
		if nonECSChild.HasNonECS || !nonECSChild.IsECS {
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

func loadFields(fileName string) []FieldDefinition {
	fs, err := loadFieldsFile(fileName)
	if err != nil {
		panic(errors.Wrapf(err, "loading module fields file failed"))
	}
	var ret []FieldDefinition
	for _, key := range fs {
		ret = append(ret, key.Fields...)
	}
	return ret
}

func loadFieldsFile(path string) ([]FieldDefinition, error) {
	fields, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return []FieldDefinition{}, nil // return empty array, this is a valid state
	}
	if err != nil {
		return nil, errors.Wrapf(err, "reading fields failed (path: %s)", path)
	}

	var fs []FieldDefinition

	err = yaml.Unmarshal(fields, &fs)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling fields file failed (path: %s)", path)
	}
	fs = loadDefaultFieldValues(fs)
	return fs, nil
}

func loadDefaultFieldValues(fs []FieldDefinition) []FieldDefinition {
	var withDefaults []FieldDefinition
	for _, f := range fs {
		if f.Type == "" {
			f.Type = "keyword"
		}
		f.Fields = loadDefaultFieldValues(f.Fields)
		withDefaults = append(withDefaults, f)
	}
	return withDefaults
}
