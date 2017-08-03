package integration_tests

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/beats/libbeat/common"
)

func TestEsDocumentation(t *testing.T) {
	assert := assert.New(t)

	fields, err := tests.LoadFields("./../_meta/fields.yml")
	assert.NoError(err)
	flattenedFieldNames := tests.FlattenFieldNames(fields, false)
	flattenedDisabledFieldNames := tests.FlattenFieldNames(fields, true)

	data, err := tests.LoadValidData("transaction")
	p := transaction.NewProcessor()
	err = p.Validate(bytes.NewReader(data))
	assert.NoError(err)
	events := p.Transform()
	eventKeys, _ := fetchEventKeys(events, flattenedDisabledFieldNames)

	undocumentedKeys, err := tests.ArrayDiff(eventKeys, flattenedFieldNames)
	assert.NoError(err)
	assert.Equal(0, len(undocumentedKeys))
}

func fetchEventKeys(events []common.MapStr, disabledKeys []string) ([]string, error) {
	keys := []string{}
	transaction := events[0]["transaction"].(common.MapStr)
	trace := events[1]["trace"].(common.MapStr)

	keys = tests.FlattenCommonMapStr(transaction, "transaction", disabledKeys, keys)
	keys = tests.FlattenCommonMapStr(trace, "trace", disabledKeys, keys)

	return keys, nil
}
