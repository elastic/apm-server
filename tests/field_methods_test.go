package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/template"
)

func TestLoadFields(t *testing.T) {
	_, err := LoadFields("non-existing")
	assert.NotNil(t, err)

	fields, err := LoadFields("./_meta/fields.yml")
	assert.Nil(t, err)
	expected := []string{"transaction", "transaction.id", "transaction.context", "exception", "exception.http", "exception.http.url", "exception.http.meta", "exception.stacktrace"}
	assert.Equal(t, expected, FlattenFieldNames(fields, false))
}

func TestFlattenFieldNames(t *testing.T) {
	assert.Equal(t, []string{}, FlattenFieldNames([]template.Field{}, true))
	assert.Equal(t, []string{}, FlattenFieldNames([]template.Field{}, false))

	fields, _ := LoadFields("./_meta/fields.yml")

	expected := []string{"transaction", "transaction.id", "transaction.context", "exception", "exception.http", "exception.http.url", "exception.http.meta", "exception.stacktrace"}
	assert.Equal(t, expected, FlattenFieldNames(fields, false))

	expected = []string{"transaction.context", "exception.stacktrace"}
	assert.Equal(t, expected, FlattenFieldNames(fields, true))
}
