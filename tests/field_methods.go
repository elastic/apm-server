package tests

import (
	"io/ioutil"

	"github.com/fatih/set"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/template"
)

func fetchFlattenedFieldNames(paths []string, addFn addField) (*set.Set, error) {
	fields := set.New()
	for _, path := range paths {
		f, err := loadFields(path)
		if err != nil {
			return nil, err
		}
		flattenFieldNames(f, "", addFn, fields)
	}
	return fields, nil
}

func flattenFieldNames(fields []template.Field, prefix string, addFn addField, flattened *set.Set) {
	for _, field := range fields {
		flattenedKey := StrConcat(prefix, field.Name, ".")
		if addFn(field) {
			flattened.Add(flattenedKey)
		}
		flattenFieldNames(field.Fields, flattenedKey, addFn, flattened)
	}
}

func loadFields(yamlPath string) ([]template.Field, error) {
	fields := []template.Field{}

	yaml, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}
	cfg, err := common.NewConfigWithYAML(yaml, "")
	if err != nil {
		return nil, err
	}
	err = cfg.Unpack(&fields)
	if err != nil {
		return nil, err
	}
	return fields, err
}

type addField func(f template.Field) bool

func addAllFields(f template.Field) bool {
	return shouldAddField(f, false)
}

func addOnlyDisabledFields(f template.Field) bool {
	return shouldAddField(f, true)
}

func addKeywordFields(f template.Field) bool {
	if f.Type == "keyword" {
		return true
	} else if len(f.MultiFields) > 0 {
		for _, mf := range f.MultiFields {
			if mf.Type == "keyword" {
				return true
			}
		}
	}
	return false
}

func shouldAddField(f template.Field, onlyDisabled bool) bool {
	if f.Name == "" {
		return false
	}
	if !onlyDisabled {
		return true
	}
	if f.Enabled != nil && *f.Enabled == false {
		return true
	}
	return false
}
