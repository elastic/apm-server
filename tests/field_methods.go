package tests

import (
	"io/ioutil"

	"github.com/fatih/set"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/template"
)

func LoadFields(yamlPath string) ([]template.Field, error) {
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

func FlattenFieldNames(fields []template.Field, onlyDisabled bool) *set.Set {
	keys := set.New()
	for _, field := range fields {
		flatten(field, "", onlyDisabled, keys)
	}
	return keys
}

func flatten(field template.Field, prefix string, onlyDisabled bool, flattened *set.Set) {
	flattenedKey := StrConcat(prefix, field.Name, ".")
	if shouldAddField(field, onlyDisabled) {
		flattened.Add(flattenedKey)
	}
	for _, f := range field.Fields {
		flatten(f, flattenedKey, onlyDisabled, flattened)
	}
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
