package tests

import (
	"testing"

	"github.com/fatih/set"
	"github.com/stretchr/testify/assert"
)

func TestLoadFields(t *testing.T) {
	_, err := loadFields("non-existing")
	assert.NotNil(t, err)

	fields, err := loadFields("./_meta/fields.yml")
	assert.Nil(t, err)
	expected := set.New("transaction", "transaction.id", "transaction.context", "exception", "exception.http", "exception.http.url", "exception.http.meta", "exception.stacktrace")
	flattened := set.New()
	flattenFieldNames(fields, "", addAllFields, flattened)
	assert.Equal(t, expected, flattened)
}

func TestFlattenFieldNames(t *testing.T) {

	fields, _ := loadFields("./_meta/fields.yml")

	expected := set.New("transaction", "transaction.id", "transaction.context", "exception", "exception.http", "exception.http.url", "exception.http.meta", "exception.stacktrace")

	flattened := set.New()
	flattenFieldNames(fields, "", addAllFields, flattened)
	assert.Equal(t, expected, flattened)

	flattened = set.New()
	flattenFieldNames(fields, "", addOnlyDisabledFields, flattened)
	expected = set.New("transaction.context", "exception.stacktrace")
	assert.Equal(t, expected, flattened)
}
