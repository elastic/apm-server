package tests

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/fatih/set"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/common"
)

func TestEventAttrsDocumentedInEsSchemaTemplate(t *testing.T, fieldPaths []string, fn processor.NewProcessor, undocumentedFieldNames *set.Set) {
	assert := assert.New(t)
	fieldNames, disabledFieldNames, err := fetchFieldNames(fieldPaths)
	fieldNames = set.Difference(fieldNames, disabledFieldNames).(*set.Set)
	assert.NoError(err)

	eventNames, err := fetchEventNames(fn, disabledFieldNames, undocumentedFieldNames)
	assert.NoError(err)

	undocumentedNames := set.Difference(eventNames, fieldNames, set.New("processor"))
	assert.Equal(0, undocumentedNames.Size(), fmt.Sprintf("Event attributes not documented in fields.yml: %v", undocumentedNames))
}

func TestEsDocumentedFieldsInEvent(t *testing.T, fieldPaths []string, fn processor.NewProcessor, undocumentedFieldNames *set.Set) {
	assert := assert.New(t)
	fieldNames, _, err := fetchFieldNames(fieldPaths)
	assert.NoError(err)

	eventNames, err := fetchEventNames(fn, set.New(), undocumentedFieldNames)
	assert.NoError(err)

	unusedNames := set.Difference(fieldNames, eventNames)
	assert.Equal(0, unusedNames.Size(), fmt.Sprintf("Documented Fields missing in event: %v", unusedNames))

}

func fetchFieldNames(paths []string) (*set.Set, *set.Set, error) {
	fields := set.New()
	fieldsDisabledIndex := set.New()
	for _, path := range paths {
		f, err := LoadFields(path)
		if err != nil {
			return nil, nil, err
		}
		fields = set.Union(fields, FlattenFieldNames(f, false)).(*set.Set)
		fieldsDisabledIndex = set.Union(fieldsDisabledIndex, FlattenFieldNames(f, true)).(*set.Set)
	}
	return fields, fieldsDisabledIndex, nil
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
			FlattenMapStr(e, k, blacklisted, eventNames)
		}
	}
	return eventNames, nil
}
