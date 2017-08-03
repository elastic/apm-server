package integration_tests

import (
	"bytes"
	"fmt"
	"testing"

	er "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/beats/libbeat/common"

	"github.com/stretchr/testify/assert"
)

func TestTemplate(t *testing.T) {
	assert := assert.New(t)

	fields, err := tests.LoadFields("./../_meta/fields.yml")
	assert.NoError(err)
	flattenedFieldNames := tests.FlattenFieldNames(fields, false)
	flattenedUndocumentedFieldNames := tests.FlattenFieldNames(fields, true)

	// Allow "tags" subkeys
	flattenedUndocumentedFieldNames = append(flattenedUndocumentedFieldNames, "error.tags")

	data, err := tests.LoadValidData("error")
	p := er.NewProcessor()
	err = p.Validate(bytes.NewReader(data))
	assert.NoError(err)
	events := p.Transform()
	eventKeys, _ := fetchEventKeys(events, flattenedUndocumentedFieldNames)

	undocumentedKeys, err := tests.ArrayDiff(eventKeys, flattenedFieldNames)
	assert.NoError(err)
	assert.Equal(0, len(undocumentedKeys), fmt.Sprintf("Undocumented keys: %+v", undocumentedKeys))
}

func fetchEventKeys(events []common.MapStr, disabledKeys []string) ([]string, error) {
	keys := []string{}
	error := events[0]["error"].(common.MapStr)

	keys = tests.FlattenCommonMapStr(error, "error", disabledKeys, keys)
	return keys, nil
}
