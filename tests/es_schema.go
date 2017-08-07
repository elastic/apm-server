package tests

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/fatih/set"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/common"
)

func TestEventAttrsDocumentedInEsSchemaTemplate(t *testing.T, fieldPaths []string, fn processor.NewProcessor, undocumentedFieldNames *set.Set) {
	assert := assert.New(t)
	fieldNames, err := fetchFlattenedFieldNames(fieldPaths, addAllFields)
	disabledFieldNames, err := fetchFlattenedFieldNames(fieldPaths, addOnlyDisabledFields)
	fieldNames = set.Difference(fieldNames, disabledFieldNames).(*set.Set)
	assert.NoError(err)

	eventNames, err := fetchEventNames(fn, disabledFieldNames, undocumentedFieldNames)
	assert.NoError(err)

	undocumentedNames := set.Difference(eventNames, fieldNames, set.New("processor"))
	assert.Equal(0, undocumentedNames.Size(), fmt.Sprintf("Event attributes not documented in fields.yml: %v", undocumentedNames))
}

func TestEsDocumentedFieldsInEvent(t *testing.T, fieldPaths []string, fn processor.NewProcessor, undocumentedFieldNames *set.Set) {
	assert := assert.New(t)
	fieldNames, err := fetchFlattenedFieldNames(fieldPaths, addAllFields)
	assert.NoError(err)

	eventNames, err := fetchEventNames(fn, set.New(), undocumentedFieldNames)
	assert.NoError(err)

	unusedNames := set.Difference(fieldNames, eventNames)
	assert.Equal(0, unusedNames.Size(), fmt.Sprintf("Documented Fields missing in event: %v", unusedNames))

}

func fetchEventNames(fn processor.NewProcessor, disabledNames *set.Set, nonIndexedNames *set.Set) (*set.Set, error) {
	p := fn()
	blacklisted := set.Union(disabledNames, nonIndexedNames).(*set.Set)
	data, _ := LoadValidData(p.Name())
	err := p.Validate(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	events := p.Transform()

	eventNames := set.New()
	for _, event := range events {
		for k, _ := range event {
			if k == "@timestamp" {
				continue
			}
			e := event[k].(common.MapStr)
			flattenMapStr(e, k, blacklisted, eventNames)
		}
	}
	return eventNames, nil
}

func flattenMapStr(m interface{}, prefix string, keysBlacklist *set.Set, flattened *set.Set) {
	if commonMapStr, ok := m.(common.MapStr); ok {
		for k, v := range commonMapStr {
			flattenMapStrStr(k, v, prefix, keysBlacklist, flattened)
		}
	} else if mapStr, ok := m.(map[string]interface{}); ok {
		for k, v := range mapStr {
			flattenMapStrStr(k, v, prefix, keysBlacklist, flattened)
		}
	}
	if prefix != "" && !isBlacklistedKey(keysBlacklist, prefix) {
		flattened.Add(prefix)
	}
}

func flattenMapStrStr(k string, v interface{}, prefix string, keysBlacklist *set.Set, flattened *set.Set) {
	flattenedKey := StrConcat(prefix, k, ".")
	if !isBlacklistedKey(keysBlacklist, flattenedKey) {
		flattened.Add(flattenedKey)
	}
	_, okCommonMapStr := v.(common.MapStr)
	_, okMapStr := v.(map[string]interface{})
	if okCommonMapStr || okMapStr {
		flattenMapStr(v, flattenedKey, keysBlacklist, flattened)
	}
}

func isBlacklistedKey(keysBlacklist *set.Set, key string) bool {
	for _, disabledKey := range keysBlacklist.List() {
		if strings.HasPrefix(key, disabledKey.(string)) {
			return true

		}
	}
	return false
}
