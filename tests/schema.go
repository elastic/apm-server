package tests

import (
	"encoding/json"
	"io"
)

type Schema struct {
	Title      string             `json:"title"`
	Properties map[string]*Schema `json:"properties"`
	Items      *Schema            `json:"items"`
}

func GetSchemaProperties(reader io.Reader) (*Schema, error) {
	decoder := json.NewDecoder(reader)
	var schema Schema
	err := decoder.Decode(&schema)
	return &schema, err
}

func FlattenSchemaProperties(s *Schema, prefix string, flattened *[]string) {
	if len(s.Properties) > 0 {
		for k, v := range s.Properties {
			flattenedKey := StrConcat(prefix, k, ".")
			*flattened = append(*flattened, flattenedKey)
			FlattenSchemaProperties(v, flattenedKey, flattened)
		}
	} else if s.Items != nil {
		FlattenSchemaProperties(s.Items, prefix, flattened)
	}
}
